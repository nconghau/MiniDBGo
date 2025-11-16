package lsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nconghau/MiniDBGo/internal/engine"
)

const (
	// Memory limits
	DefaultFlushSize     = 10000            // records before flush
	DefaultMemTableBytes = 50 * 1024 * 1024 // 50MB max per memtable
	MaxImmutableTables   = 3                // max concurrent immutable tables

	// Timeouts
	FlushTimeout    = 30 * time.Second
	CompactTimeout  = 5 * time.Minute
	ShutdownTimeout = 10 * time.Second

	// --- MỚI: Cấu hình Compaction ---
	L0CompactionTrigger = 4 // Kích hoạt nén L0 -> L1 khi có 4 tệp L0
)

type LSMEngine struct {
	dir      string
	wal      *WAL
	mem      *MemTable //
	memBytes int64

	immutMu    sync.RWMutex
	immutables []*MemTable

	sstDir      string
	seq         int
	flushSize   int64
	maxMemBytes int64

	mu           sync.RWMutex // Bảo vệ 'current', 'seq', 'wal', 'mem'
	shuttingDown bool

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Flush management
	flushCh  chan *MemTable
	flushErr atomic.Value

	// Metrics
	metrics struct {
		puts     atomic.Int64
		gets     atomic.Int64
		deletes  atomic.Int64
		flushes  atomic.Int64
		compacts atomic.Int64
	}

	// --- MỚI: Quản lý Version và Compaction ---
	manifestPath string
	current      *Version
	compactionCh chan struct{} // Channel để kích hoạt nén
	compactMu    sync.Mutex    // Đảm bảo chỉ 1 compaction chạy
}

// --- MỚI: KIỂM TRA STATIC ---
// Dòng này sẽ biên dịch thành công
var _ engine.Engine = (*LSMEngine)(nil)

// --- SỬA ĐỔI: Kiểu trả về là engine.Engine ---
func OpenLSM(dir string) (engine.Engine, error) {
	return OpenLSMWithConfig(dir, DefaultFlushSize, DefaultMemTableBytes)
}

// --- SỬA ĐỔI: Kiểu trả về là engine.Engine ---
func OpenLSMWithConfig(dir string, flushSize int64, maxMemBytes int64) (engine.Engine, error) {
	// ... (logic [cite: 187-193] gốc giữ nguyên) ...
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}
	walDir := filepath.Join(dir, "wal")
	sstDir := filepath.Join(dir, "sst")
	if err := os.MkdirAll(walDir, 0o755); err != nil {
		return nil, fmt.Errorf("create wal dir: %w", err)
	}
	if err := os.MkdirAll(sstDir, 0o755); err != nil {
		return nil, fmt.Errorf("create sst dir: %w", err)
	}
	manifestPath := filepath.Join(dir, manifestFileName)
	currentVersion, err := loadManifest(dir)
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}
	seq := 1
	for _, files := range currentVersion.Levels {
		for _, f := range files {
			var l, s int
			fmt.Sscanf(filepath.Base(f.Path), "sst-L%d-%d.sst", &l, &s)
			if s >= seq {
				seq = s + 1
			}
		}
	}
	w, err := OpenWAL(walDir, seq)
	if err != nil {
		return nil, fmt.Errorf("open wal: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	engine := &LSMEngine{
		dir: dir, wal: w, mem: NewMemTable(), immutables: make([]*MemTable, 0, MaxImmutableTables),
		sstDir: sstDir, seq: seq, flushSize: flushSize, maxMemBytes: maxMemBytes, ctx: ctx, cancel: cancel,
		flushCh: make(chan *MemTable, MaxImmutableTables), manifestPath: manifestPath, current: currentVersion,
		compactionCh: make(chan struct{}, 1),
	}
	replayedFiles, err := engine.replayWAL(walDir)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("replay wal: %w", err)
	}
	if engine.mem.Size() > 0 {
		slog.Info("Flushing replayed WAL data to SSTable...", "count", engine.mem.Size())
		if err := engine.flushMemTable(engine.mem); err != nil {
			cancel()
			return nil, fmt.Errorf("failed to flush replayed data: %w", err)
		}
		engine.mem = NewMemTable()
		atomic.StoreInt64(&engine.memBytes, 0)
		for _, p := range replayedFiles {
			if err := os.Remove(p); err != nil {
				slog.Warn("Failed to delete replayed WAL file", "path", p, "error", err)
			}
		}
		slog.Info("Cleaned up replayed WAL files.", "count", len(replayedFiles))
	}
	engine.wg.Add(2)
	go engine.flushWorker()
	go engine.compactionWorker()
	return engine, nil
}

// (replayWAL, flushWorker, flushMemTable, removeImmutable... giữ nguyên như cũ)
// ... (Bỏ qua các hàm không thay đổi để tiết kiệm không gian) ...
func (e *LSMEngine) replayWAL(walDir string) ([]string, error) {
	walFiles, err := os.ReadDir(walDir)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0)
	for _, f := range walFiles {
		if strings.HasPrefix(f.Name(), "wal-") && strings.HasSuffix(f.Name(), ".log") {
			names = append(names, filepath.Join(walDir, f.Name()))
		}
	}
	sort.Strings(names)

	for _, p := range names {
		tmpF, err := os.Open(p)
		if err != nil {
			continue
		}

		wr := &WAL{f: tmpF, path: p}
		_ = wr.Iterate(func(flags byte, key, value []byte) error {
			k := string(key) // [cite: 149]
			if flags == 1 {
				e.mem.Delete(k)
			} else {
				e.mem.Put(k, value)
				atomic.AddInt64(&e.memBytes, int64(len(key)+len(value)))
			}
			return nil
		})
		tmpF.Close()
	}

	return names, nil
}

// flushWorker
// --- SỬA ĐỔI: Chỉ gọi flushMemTable ---
func (e *LSMEngine) flushWorker() {
	defer e.wg.Done()
	slog.Info("Flush worker started", "component", "lsm")

	for memTable := range e.flushCh {
		slog.Info("Starting memtable flush", "component", "lsm")
		start := time.Now()

		if err := e.flushMemTable(memTable); err != nil { //
			e.flushErr.Store(err)
			slog.Error("Memtable flush error",
				"component", "lsm",
				"error", err,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		} else {
			slog.Info("Memtable flush complete",
				"component", "lsm",
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}

		// --- MỚI: Kích hoạt kiểm tra compaction ---
		e.tryScheduleCompaction()
	}
	slog.Info("Flush worker stopped (channel closed).", "component", "lsm")
}

// flushMemTable
// --- SỬA ĐỔI: Cập nhật Manifest thay vì chỉ xóa immutable ---
func (e *LSMEngine) flushMemTable(memTable *MemTable) error {
	ctx, cancel := context.WithTimeout(e.ctx, FlushTimeout)
	defer cancel()

	items := memTable.SnapshotAndReset()
	if len(items) == 0 {
		e.removeImmutable(memTable) // Vẫn xóa khỏi danh sách immutable
		return nil
	}

	// ... (kiểm tra context) ...
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 1. Lấy seq và tăng (cần khóa mu)
	e.mu.Lock()
	seq := e.seq
	e.seq++
	e.mu.Unlock()

	// 2. Viết SSTable (Level 0)
	path := filepath.Join(e.sstDir, fmt.Sprintf("sst-L0-%06d.sst", seq))
	writer, err := NewSSTWriter(path, uint32(len(items)))
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if err := writer.WriteEntry(key, items[key]); err != nil {
			writer.Close()
			os.Remove(path)
			return err
		}
	}
	if err := writer.Close(); err != nil {
		os.Remove(path)
		return err
	}

	// 3. Cập nhật Manifest (cần khóa mu)
	meta := writer.GetMetadata()
	fileMeta := &FileMetadata{
		Level:    0,
		Path:     path,
		MinKey:   meta.MinKey,
		MaxKey:   meta.MaxKey,
		FileSize: meta.FileSize,
		KeyCount: meta.KeyCount,
	}

	e.mu.Lock()
	e.current.AddFile(fileMeta)
	err = e.saveManifest() // Ghi đè MANIFEST
	e.mu.Unlock()

	if err != nil {
		// Lỗi nghiêm trọng: SST đã được viết nhưng MANIFEST lỗi
		slog.Error("CRITICAL: Failed to save manifest after flush", "error", err)
		// (Trong CSDL thực, chúng ta sẽ thử lại)
		return err
	}

	// 4. Dọn dẹp
	e.removeImmutable(memTable)
	e.metrics.flushes.Add(1)
	return nil
}

// --- MỚI: Các hàm Compaction ---

// compactionWorker là goroutine chạy nền
func (e *LSMEngine) compactionWorker() {
	defer e.wg.Done()
	slog.Info("Compaction worker started", "component", "lsm")

	for range e.compactionCh {
		if e.ctx.Err() != nil {
			break // Engine đang tắt
		}

		// Khóa compactMu đảm bảo chỉ 1 compaction chạy
		e.compactMu.Lock()

		slog.Info("Compaction triggered", "component", "lsm")
		start := time.Now()

		if err := e.runCompaction(); err != nil {
			slog.Error("Compaction error", "error", err)
		} else {
			slog.Info("Compaction finished", "duration_ms", time.Since(start).Milliseconds())
		}

		e.compactMu.Unlock()

		// Kích hoạt kiểm tra lại, phòng khi L0
		// lại đầy trong lúc đang nén
		e.tryScheduleCompaction()
	}

	slog.Info("Compaction worker stopped.", "component", "lsm")
}

// tryScheduleCompaction kiểm tra xem có cần nén không
// và gửi tín hiệu không-chặn (non-blocking)
func (e *LSMEngine) tryScheduleCompaction() {
	e.mu.RLock()
	// --- SỬA ĐỔI: Kiểm tra cờ trước khi gửi ---
	if e.shuttingDown {
		e.mu.RUnlock()
		return // Đang tắt, không kích hoạt nữa
	}

	// Chính sách: Nén L0 nếu có >= N tệp
	needsCompaction := len(e.current.Levels[0]) >= L0CompactionTrigger
	e.mu.RUnlock() // Mở khóa

	if needsCompaction {
		select {
		case e.compactionCh <- struct{}{}:
			// Đã gửi tín hiệu
		default:
			// Worker đã bận, không cần gửi nữa
		}
	}
}

// --- KẾT THÚC MÃ MỚI ---

// Compact (API công khai) chỉ kích hoạt
// một lần kiểm tra nén nền (non-blocking).
// Đây là hàm triển khai engine.Engine.
func (e *LSMEngine) Compact() error {
	e.tryScheduleCompaction()
	return nil
}

// --- KẾT THÚC SỬA LỖI ---

func (e *LSMEngine) removeImmutable(memTable *MemTable) {
	e.immutMu.Lock()
	defer e.immutMu.Unlock()
	for i, m := range e.immutables {
		if m == memTable {
			e.immutables = append(e.immutables[:i], e.immutables[i+1:]...)
			break
		}
	}
}

// --- SỬA ĐỔI: Triển khai engine.Engine ---
func (e *LSMEngine) NewBatch() engine.Batch {
	return NewBatch() // (Hàm NewBatch() trong lsm/batch.go)
}

func (e *LSMEngine) ApplyBatch(b engine.Batch) error {
	lsmBatch, ok := b.(*lsmBatch) // Ép kiểu
	if !ok {
		return errors.New("invalid batch type provided")
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ctx.Err() != nil {
		return errors.New("engine is shutting down")
	} // [cite: 196-197]
	if lsmBatch.Size() == 0 {
		return nil
	}

	for _, entry := range lsmBatch.entries {
		if err := e.wal.Append(entry.Key, entry.Value, entry.Tombstone); err != nil { // [cite: 197-198]
			return fmt.Errorf("wal append batch: %w", err)
		}
	}

	needsFlush := false
	for _, entry := range lsmBatch.entries {
		k := string(entry.Key)
		if entry.Tombstone {
			e.mem.Delete(k)
			atomic.AddInt64(&e.memBytes, int64(len(k)))
		} else {
			e.mem.Put(k, entry.Value)
			atomic.AddInt64(&e.memBytes, int64(len(k)+len(entry.Value)))
		}
		if e.mem.Size() >= e.flushSize || atomic.LoadInt64(&e.memBytes) >= e.maxMemBytes { // [cite: 198-199]
			needsFlush = true
		}
	}

	if needsFlush {
		if err := e.rotateMemTable(); err != nil { // [cite: 199-201]
			return fmt.Errorf("rotate memtable: %w", err)
		}
	}
	return nil
}

// --- TÁI CẤU TRÚC (REFACTOR) Put và Delete ---

func (e *LSMEngine) Put(key, value []byte) error {
	e.metrics.puts.Add(1)

	// --- SỬA ĐỔI: Sử dụng ApplyBatch ---
	// Logic cũ [cite: 151-153] được thay thế
	b := NewBatch()
	b.Put(key, value)
	return e.ApplyBatch(b)
}

func (e *LSMEngine) Update(key, value []byte) error {
	return e.Put(key, value)
}

func (e *LSMEngine) Delete(key []byte) error {
	e.metrics.deletes.Add(1)

	// --- SỬA ĐỔI: Sử dụng ApplyBatch ---
	// Logic cũ [cite: 154-155] được thay thế
	b := NewBatch()
	b.Delete(key)
	return e.ApplyBatch(b)
}

// --- KẾT THÚC TÁI CẤU TRÚC ---

// Get
// --- SỬA ĐỔI: Đọc từ Version (Levels) ---
func (e *LSMEngine) Get(key []byte) ([]byte, error) { //
	e.metrics.gets.Add(1)
	k := string(key)

	// 1. Check active memtable
	e.mu.RLock()
	if it, ok := e.mem.Get(k); ok { //
		e.mu.RUnlock()
		if it.Tombstone {
			return nil, errors.New("key not found")
		}
		return it.Value, nil
	}
	e.mu.RUnlock()

	// 2. Check immutable memtables
	e.immutMu.RLock()
	for _, m := range e.immutables {
		if it, ok := m.Get(k); ok { //
			e.immutMu.RUnlock()
			if it.Tombstone {
				return nil, errors.New("key not found")
			}
			return it.Value, nil
		}
	}
	e.immutMu.RUnlock()

	// 3. Search SST files (từ Version)
	e.mu.RLock()
	l0Files := e.current.Levels[0]
	l1Files := e.current.Levels[1]
	e.mu.RUnlock()

	// 3a. Quét L0 (mới nhất -> cũ nhất)
	// (L0 có thể chồng lấn, phải quét hết)
	for i := len(l0Files) - 1; i >= 0; i-- {
		meta := l0Files[i]
		if k < meta.MinKey || k > meta.MaxKey {
			continue // Tối ưu hóa: Bỏ qua tệp nếu key nằm ngoài phạm vi
		}
		if bv, tomb, err := ReadSSTFind(meta.Path, k); err == nil { //
			if tomb {
				return nil, errors.New("key not found")
			}
			if bv != nil {
				return bv, nil
			}
		}
	}

	// 3b. Quét L1 (đã sắp xếp, không chồng lấn)
	for _, meta := range l1Files {
		if k >= meta.MinKey && k <= meta.MaxKey {
			// Vì L1 không chồng lấn, nếu key nằm trong
			// phạm vi, nó PHẢI ở đây (hoặc không tồn tại)
			if bv, tomb, err := ReadSSTFind(meta.Path, k); err == nil {
				if tomb {
					return nil, errors.New("key not found")
				}
				if bv != nil {
					return bv, nil
				}
			}
			// Nếu ReadSSTFind lỗi (os.ErrNotExist),
			// chúng ta có thể dừng tìm L1
			break
		}
	}

	return nil, errors.New("key not found")
}

// --- KẾT THÚC SỬA ĐỔI ---

// NewIterator
// --- SỬA ĐỔI: Đọc từ Version (Levels) ---
func (e *LSMEngine) NewIterator() (engine.Iterator, error) {
	e.mu.RLock()
	e.immutMu.RLock()

	iters := make([]engine.Iterator, 0, len(e.immutables)+10)

	// 1. Thêm MemTable (mới nhất)
	iters = append(iters, NewMemTableIterator(e.mem))

	// 2. Thêm Immutables (thứ tự mới -> cũ)
	for i := len(e.immutables) - 1; i >= 0; i-- { //
		iters = append(iters, NewMemTableIterator(e.immutables[i]))
	}

	e.immutMu.RUnlock()

	// 3. Thêm SSTables (từ Version)
	l0Files := e.current.Levels[0]
	l1Files := e.current.Levels[1]
	e.mu.RUnlock() // Mở khóa chính

	// 3a. L0 (mới nhất -> cũ nhất)
	for i := len(l0Files) - 1; i >= 0; i-- {
		it, err := NewSSTableIterator(l0Files[i].Path)
		if err != nil {
			for _, it := range iters {
				it.Close()
			}
			return nil, fmt.Errorf("open sst L0 iterator: %w", err)
		}
		iters = append(iters, it)
	}

	// 3b. L1 (sắp xếp theo key)
	for _, meta := range l1Files {
		it, err := NewSSTableIterator(meta.Path)
		if err != nil {
			for _, it := range iters {
				it.Close()
			}
			return nil, fmt.Errorf("open sst L1 iterator: %w", err)
		}
		iters = append(iters, it)
	}

	// 5. Trả về MergingIterator
	return NewMergingIterator(iters), nil
}

// ... (Các hàm IterKeys, streamSSTKeys, mapToSlice, rotateMemTable, DumpDB, RestoreDB, Close, GetMetrics giữ nguyên) ...
// (Bỏ qua các hàm không thay đổi để tiết kiệm không gian)
func (e *LSMEngine) IterKeys() ([]string, error) {
	return e.IterKeysWithLimit(0) // 0 = no limit, but still stream
}

// IterKeysWithLimit
// --- SỬA ĐỔI: Viết lại hoàn toàn để dùng Iterator ---
func (e *LSMEngine) IterKeysWithLimit(limit int) ([]string, error) {
	keysMap := make(map[string]struct{})
	count := 0

	it, err := e.NewIterator()
	if err != nil {
		return nil, fmt.Errorf("new iterator: %w", err)
	}
	// Đảm bảo iterator được đóng
	defer it.Close()

	for it.Next() {
		if limit > 0 && count >= limit {
			break
		}

		key := it.Key()
		if _, exists := keysMap[key]; !exists {
			keysMap[key] = struct{}{}
			count++
		}
	}

	if err := it.Error(); err != nil {
		return nil, fmt.Errorf("iterator error: %w", err)
	}

	return mapToSlice(keysMap), nil
}

// --- KẾT THÚC SỬA ĐỔI ---

func mapToSlice(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (e *LSMEngine) rotateMemTable() error {
	// Check if too many immutable tables
	e.immutMu.RLock()
	immutableCount := len(e.immutables)
	e.immutMu.RUnlock()

	if immutableCount >= MaxImmutableTables {
		// Block until flush completes to prevent unbounded memory growth
		return errors.New("too many pending flushes, please retry")
	}

	// Snapshot current memtable
	snap := e.mem
	e.mem = NewMemTable()
	atomic.StoreInt64(&e.memBytes, 0)

	// Add to immutables
	e.immutMu.Lock()
	e.immutables = append(e.immutables, snap)
	e.immutMu.Unlock()

	// Send to flush worker (non-blocking with timeout)
	select {
	case e.flushCh <- snap:
		// Success
	case <-time.After(time.Second):
		return errors.New("flush queue full")
	}

	return nil
}

// DumpDB
// --- SỬA ĐỔI: Viết lại hoàn toàn để dùng Iterator ---
func (e *LSMEngine) DumpDB(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err // [cite: 167]
	}
	defer f.Close()

	enc := json.NewEncoder(f)

	// Sử dụng iterator để quét toàn bộ CSDL
	it, err := e.NewIterator()
	if err != nil {
		return err
	}
	defer it.Close()

	collections := make(map[string][]map[string]interface{})

	for it.Next() {
		fullKey := it.Key()
		idx := strings.Index(fullKey, ":")
		if idx < 0 {
			continue // Bỏ qua key không hợp lệ
		}

		col := fullKey[:idx]
		id := fullKey[idx+1:]

		v := it.Value().Value // Lấy giá trị trực tiếp từ iterator
		if v == nil {
			continue
		}

		var doc map[string]interface{}
		if err := json.Unmarshal(v, &doc); err != nil { // [cite: 169]
			continue // Bỏ qua JSON không hợp lệ
		}

		doc["_id"] = id // Đảm bảo _id luôn đúng
		collections[col] = append(collections[col], doc)
	}

	if err := it.Error(); err != nil {
		return err
	}

	// Logic [cite: 168] cũ đã được thay thế
	return enc.Encode(collections)
}

// --- KẾT THÚC SỬA ĐỔI ---

func (e *LSMEngine) RestoreDB(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Stream decode to avoid loading entire file into memory
	dec := json.NewDecoder(f)
	var data map[string][]map[string]interface{}
	if err := dec.Decode(&data); err != nil { // [cite: 170]
		return err
	}
	for col, docs := range data {
		for _, doc := range docs {
			idV, ok := doc["_id"]
			if !ok {
				return fmt.Errorf("missing _id in doc for collection %s", col)
			}
			idStr, ok := idV.(string)
			if !ok {
				return fmt.Errorf("_id must be string")
			}
			raw, _ := json.Marshal(doc)
			if err := e.Put([]byte(col+":"+idStr), raw); err != nil { // [cite: 171]
				return err
			}
		}
		// Clear docs to free memory between collections
		data[col] = nil
	}
	return nil
}

func (e *LSMEngine) Close() error {
	slog.Info("Database closing...", "component", "lsm")

	// --- SỬA ĐỔI: Set cờ shuttingDown ---
	e.mu.Lock()
	if e.shuttingDown {
		e.mu.Unlock()
		return errors.New("database already closing")
	}
	e.shuttingDown = true
	e.mu.Unlock()
	// --- KẾT THÚC SỬA ĐỔI ---

	// 1. Đẩy nốt dữ liệu RAM vào hàng đợi (nếu có)
	if e.mem.Size() > 0 {
		slog.Info("Scheduling active MemTable flush before shutdown...", "component", "lsm")
		if err := e.rotateMemTable(); err != nil { // [cite: 214-215]
			slog.Error("Failed to schedule final MemTable flush on close", "error", err)
		}
	}

	// 2. Đóng flushCh
	close(e.flushCh)

	// 3. Đóng compactionCh
	close(e.compactionCh)

	// 4. Chờ worker
	e.wg.Wait()
	slog.Info("All workers finished.", "component", "lsm")

	e.cancel()

	// 5. Đóng WAL
	if e.wal != nil {
		if err := e.wal.Close(); err != nil { //
			return err
		}
	}
	slog.Info("Database closed gracefully.", "component", "lsm")
	return nil
}

// --- KẾT THÚC SỬA ĐỔI ---

func (e *LSMEngine) GetMetrics() map[string]int64 {
	return map[string]int64{
		"puts":     e.metrics.puts.Load(),
		"gets":     e.metrics.gets.Load(),
		"deletes":  e.metrics.deletes.Load(),
		"flushes":  e.metrics.flushes.Load(),
		"compacts": e.metrics.compacts.Load(),
	}
}
