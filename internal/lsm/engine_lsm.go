package lsm

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
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

// ErrCorruption là lỗi trả về khi phát hiện
// dữ liệu trên đĩa bị hỏng (checksum không khớp).
var ErrCorruption = errors.New("data corruption detected")
var crcTable = crc32.MakeTable(crc32.Castagnoli)

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
	// Kích hoạt nén L1 -> L2 khi L1 vượt quá 100MB
	L1CompactionTriggerBytes = 100 * 1024 * 1024
)

type flushTask struct {
	mem     *MemTable
	walPath string // Đường dẫn file WAL cần xóa sau khi flush xong
}

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
	flushCh  chan flushTask
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
		dir: dir, wal: w, mem: NewMemTable(),
		immutables:   make([]*MemTable, 0, MaxImmutableTables),
		sstDir:       sstDir,
		seq:          seq,
		flushSize:    flushSize,
		maxMemBytes:  maxMemBytes,
		ctx:          ctx,
		cancel:       cancel,
		flushCh:      make(chan flushTask, MaxImmutableTables),
		manifestPath: manifestPath, current: currentVersion,
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

		// --- BẮT ĐẦU MÃ MỚI ---
		// SAU KHI FLUSH, ĐÁNH THỨC COMPACTION WORKER ĐỂ NÓ KIỂM TRA
		engine.tryScheduleCompaction()
		// --- KẾT THÚC MÃ MỚI ---
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

func (e *LSMEngine) flushWorker() {
	defer e.wg.Done()
	slog.Info("Flush worker started", "component", "lsm")

	// Sửa vòng lặp nhận task
	for task := range e.flushCh {
		slog.Info("Starting memtable flush", "component", "lsm")
		start := time.Now()

		// Gọi flushMemTable với task.mem
		if err := e.flushMemTable(task.mem); err != nil {
			e.flushErr.Store(err)
			slog.Error("Memtable flush error", "error", err)
		} else {
			// --- FIX: Flush thành công -> Xóa file WAL cũ ---
			if task.walPath != "" {
				if err := os.Remove(task.walPath); err != nil {
					slog.Warn("Failed to remove old WAL", "path", task.walPath, "error", err)
				} else {
					slog.Debug("Removed old WAL file", "path", task.walPath)
				}
			}
			// ------------------------------------------------

			slog.Info("Memtable flush complete", "duration_ms", time.Since(start).Milliseconds())
		}

		e.tryScheduleCompaction()
	}
	slog.Info("Flush worker stopped (channel closed).", "component", "lsm")
}

// flushMemTable
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

		if err := e.pickAndRunCompaction(); err != nil {
			slog.Error("Compaction error", "error", err)
		}
	}

	slog.Info("Compaction worker stopped.", "component", "lsm")
}

// --- BẮT ĐẦU MÃ MỚI ---
// (Thêm hàm mới này vào file engine_lsm.go)

// pickAndRunCompaction là bộ não mới: nó quyết định CÓ
// cần nén không, và nén CẤP NÀO.
func (e *LSMEngine) pickAndRunCompaction() error {
	e.compactMu.Lock() // Khóa để đảm bảo chỉ 1 compaction chạy
	defer e.compactMu.Unlock()

	// Lấy snapshot của version hiện tại
	e.mu.RLock()
	l0Files := e.current.Levels[0]
	l1Files := e.current.Levels[1]
	// Chúng ta cần lấy l2Files ngay cả khi nó không tồn tại
	// để dùng trong logic tìm file chồng lấn (overlap)
	l2Files := e.current.Levels[2]
	e.mu.RUnlock()

	// --- Quyết định 1: Ưu tiên L0 ---
	if len(l0Files) >= L0CompactionTrigger {
		slog.Info("Starting L0->L1 compaction | pickAndRunCompaction", "files", len(l0Files))
		// (Chúng ta sẽ đổi tên hàm runCompaction() thành runL0Compaction)
		return e.runL0Compaction(l0Files)
	}

	// --- Quyết định 2: Kiểm tra L1 ---
	var l1Size int64
	for _, f := range l1Files {
		l1Size += f.FileSize
	}

	if l1Size > L1CompactionTriggerBytes {
		slog.Info("Starting L1->L2 compaction", "l1_size_mb", l1Size/1024/1024)
		// (Đây là hàm mới chúng ta sắp viết)
		return e.runL1Compaction(l1Files, l2Files)
	}

	slog.Debug("No compaction needed")
	return nil
}

// --- KẾT THÚC MÃ MỚI ---

// (Hàm này đã có, chỉ cần sửa logic kiểm tra L1)
func (e *LSMEngine) tryScheduleCompaction() {
	e.mu.RLock()
	if e.shuttingDown {
		e.mu.RUnlock()
		return
	}

	// Chính sách: Nén L0 nếu có >= N tệp
	needsL0Compaction := len(e.current.Levels[0]) >= L0CompactionTrigger

	// --- BẮT ĐẦU MÃ MỚI ---
	// Chính sách: Nén L1 nếu kích thước > L1CompactionTriggerBytes
	var l1Size int64
	for _, f := range e.current.Levels[1] {
		l1Size += f.FileSize
	}
	needsL1Compaction := l1Size > L1CompactionTriggerBytes
	// --- KẾT THÚC MÃ MỚI ---

	e.mu.RUnlock() // Mở khóa

	// Chỉ cần một trong hai điều kiện là đủ để "đánh thức" worker
	if needsL0Compaction || needsL1Compaction {
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
	if e.shuttingDown {
		return errors.New("database is shutting down")
	}
	if e.ctx.Err() != nil {
		return errors.New("engine is shutting down")
	}
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
	b := NewBatch()
	b.Delete(key)
	return e.ApplyBatch(b)
}

// Get
func (e *LSMEngine) Get(key []byte) ([]byte, error) {
	e.metrics.gets.Add(1)
	k := string(key)

	// 1. Check active memtable
	e.mu.RLock()
	if it, ok := e.mem.Get(k); ok {
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
		if it, ok := m.Get(k); ok {
			e.immutMu.RUnlock()
			if it.Tombstone {
				return nil, errors.New("key not found")
			}
			return it.Value, nil
		}
	}
	e.immutMu.RUnlock()

	// 3. Search SST files (L0 -> LMax)
	e.mu.RLock()
	// Copy snapshot của levels để nhả lock sớm
	levelsSnapshot := make(map[int][]*FileMetadata)
	for level, files := range e.current.Levels {
		levelsSnapshot[level] = files
	}
	e.mu.RUnlock()

	// 3a. Quét L0 (Đặc biệt: có chồng lấn, phải quét từ Mới -> Cũ)
	if l0Files, ok := levelsSnapshot[0]; ok {
		for i := len(l0Files) - 1; i >= 0; i-- {
			meta := l0Files[i]
			if k < meta.MinKey || k > meta.MaxKey {
				continue
			}
			// --- [FIX 1] Xử lý lỗi chuẩn cho L0 ---
			bv, tomb, err := ReadSSTFind(meta.Path, k)
			if err == nil {
				// Tìm thấy!
				if tomb {
					return nil, errors.New("key not found")
				}
				if bv != nil {
					return bv, nil
				}
			} else if err != os.ErrNotExist {
				// Lỗi hệ thống (IO, Checksum...), log warning nhưng không return lỗi ngay
				// để hệ thống cố gắng tìm ở các file cũ hơn (Hy vọng có bản backup)
				slog.Warn("Error reading L0 SST", "path", meta.Path, "error", err)
			}
			// Nếu err == os.ErrNotExist -> Chỉ đơn giản là không có, loop tiếp.
		}
	}

	// 3b. Quét L1 trở đi (L1, L2, L3...: Không chồng lấn trong cùng 1 level)
	// Chúng ta lặp qua các Level có sẵn (1, 2, 3...)
	// Lưu ý: Cần sort level key để đảm bảo thứ tự 1 -> 2 -> 3
	var sortedLevels []int
	for level := range levelsSnapshot {
		if level > 0 {
			sortedLevels = append(sortedLevels, level)
		}
	}
	sort.Ints(sortedLevels)

	for _, level := range sortedLevels {
		files := levelsSnapshot[level]
		// Với Level >= 1, các file đã sort và không overlap.
		// Chúng ta dùng Binary Search hoặc duyệt tuần tự check Min/Max
		for _, meta := range files {
			if k >= meta.MinKey && k <= meta.MaxKey {
				// Key nằm trong phạm vi file này.
				// Vì không overlap, nếu key tồn tại ở Level này, nó CHỈ có thể ở file này.
				// --- [FIX 2] Xử lý lỗi chuẩn cho Level > 0 ---
				bv, tomb, err := ReadSSTFind(meta.Path, k)
				if err == nil {
					if tomb {
						return nil, errors.New("key not found")
					}
					if bv != nil {
						return bv, nil
					}
				} else if err != os.ErrNotExist {
					// Log warning nếu file bị hỏng
					slog.Warn("Error reading SST Level > 0", "level", level, "path", meta.Path, "error", err)
				}

				// Logic quan trọng của LSM Level > 0:
				// Vì các file không overlap, nếu key nằm trong Range [Min, Max] của file này
				// mà tìm trong file không thấy (hoặc file lỗi), thì CHẮC CHẮN key không tồn tại ở Level này.
				// Ta break để xuống Level sâu hơn tìm tiếp.
				goto NextLevel
			}
		}
	NextLevel:
	}

	return nil, errors.New("key not found")
}

// --- KẾT THÚC SỬA ĐỔI ---

// NewIterator
func (e *LSMEngine) NewIterator() (engine.Iterator, error) {
	e.mu.RLock()
	e.immutMu.RLock()

	// Dự kiến số lượng iterator
	iters := make([]engine.Iterator, 0, len(e.immutables)+10)

	// 1. Thêm MemTable
	iters = append(iters, NewMemTableIterator(e.mem))

	// 2. Thêm Immutables
	for i := len(e.immutables) - 1; i >= 0; i-- {
		iters = append(iters, NewMemTableIterator(e.immutables[i]))
	}
	e.immutMu.RUnlock()

	// 3. Snapshot Levels
	levelsSnapshot := make(map[int][]*FileMetadata)
	for level, files := range e.current.Levels {
		levelsSnapshot[level] = files
	}
	e.mu.RUnlock()

	// 4. Thêm L0 (Mới -> Cũ)
	if l0Files, ok := levelsSnapshot[0]; ok {
		for i := len(l0Files) - 1; i >= 0; i-- {
			it, err := NewSSTableIterator(l0Files[i].Path)
			if err != nil {
				// Close opened iters -> handle error cleanup carefully in prod
				return nil, fmt.Errorf("open sst L0 iterator: %w", err)
			}
			iters = append(iters, it)
		}
	}

	// 5. Thêm L1, L2... (Sorted Levels)
	var sortedLevels []int
	for level := range levelsSnapshot {
		if level > 0 {
			sortedLevels = append(sortedLevels, level)
		}
	}
	sort.Ints(sortedLevels)

	for _, level := range sortedLevels {
		files := levelsSnapshot[level]
		// Level > 0: Các file không overlap, nhưng ta vẫn cần add tất cả vào
		// MergingIterator để nó merge đúng thứ tự key toàn cục.
		// (Hoặc tối ưu hơn là dùng ConcatIterator cho mỗi Level, nhưng Merging vẫn chạy đúng)
		for _, meta := range files {
			it, err := NewSSTableIterator(meta.Path)
			if err != nil {
				return nil, fmt.Errorf("open sst L%d iterator: %w", level, err)
			}
			iters = append(iters, it)
		}
	}

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

// internal/lsm/engine_lsm.go

func (e *LSMEngine) rotateMemTable() error {
	e.immutMu.RLock()
	immutableCount := len(e.immutables)
	e.immutMu.RUnlock()

	if immutableCount >= MaxImmutableTables {
		return errors.New("too many pending flushes, please retry")
	}

	// 1. Đóng WAL hiện tại
	oldWALPath := e.wal.path // Lưu đường dẫn để xóa sau
	if err := e.wal.Close(); err != nil {
		return fmt.Errorf("close wal: %w", err)
	}

	// 2. Tạo WAL mới
	// Lưu ý: seq của engine dùng cho SST, ta có thể dùng timestamp hoặc seq riêng cho WAL.
	// Để đơn giản và tránh conflict, dùng Seq hiện tại + Nano time
	newWalPath := filepath.Join(e.dir, "wal", fmt.Sprintf("wal-%d-%d.log", e.seq, time.Now().UnixNano()))
	newWalFile, err := os.OpenFile(newWalPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("create new wal: %w", err)
	}

	// Cập nhật e.wal trỏ tới file mới
	// (Lưu ý: Cần sửa struct WAL để public field hoặc tạo hàm NewWAL linh hoạt hơn,
	// nhưng ở đây tôi giả định bạn fix nhanh bằng cách gán lại struct)
	e.wal = &WAL{
		f:    newWalFile,
		path: newWalPath,
		w:    bufio.NewWriterSize(newWalFile, 256*1024),
	}

	// 3. Snapshot Memtable
	snap := e.mem
	e.mem = NewMemTable()
	atomic.StoreInt64(&e.memBytes, 0)

	// 4. Add to immutables
	e.immutMu.Lock()
	e.immutables = append(e.immutables, snap)
	e.immutMu.Unlock()

	// 5. Gửi cả Memtable và OldWALPath vào channel
	task := flushTask{
		mem:     snap,
		walPath: oldWALPath,
	}

	select {
	case e.flushCh <- task:
		return nil
	case <-time.After(time.Second):
		return errors.New("flush queue full")
	}
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
	// 1. Lấy các counters (bộ đếm) cũ (như hiện tại)
	metricsMap := map[string]int64{
		"puts":     e.metrics.puts.Load(),
		"gets":     e.metrics.gets.Load(),
		"deletes":  e.metrics.deletes.Load(),
		"flushes":  e.metrics.flushes.Load(),
		"compacts": e.metrics.compacts.Load(),
	}

	// --- BẮT ĐẦU MÃ MỚI ---
	// 2. Lấy các gauges (trạng thái) về bộ nhớ
	// (Cần khóa RLock để đọc an toàn)
	e.mu.RLock()
	metricsMap["memtable_entries"] = e.mem.Size()
	metricsMap["memtable_bytes"] = e.mem.ByteSize()
	e.mu.RUnlock()

	e.immutMu.RLock()
	metricsMap["immutable_count"] = int64(len(e.immutables))
	e.immutMu.RUnlock()

	// 3. Lấy các gauges về đĩa (trạng thái các Cấp)
	// (Đây là phần quan trọng nhất)
	e.mu.RLock()
	// Sao chép map Levels để tránh giữ khóa lâu
	levelsSnapshot := make(map[int][]*FileMetadata)
	for level, files := range e.current.Levels {
		levelsSnapshot[level] = files // Chỉ sao chép slice header
	}
	e.mu.RUnlock()

	// Khởi tạo tất cả các cấp (L0, L1, L2) để chúng luôn xuất hiện
	metricsMap["level_0_files"] = 0
	metricsMap["level_0_bytes"] = 0
	metricsMap["level_1_files"] = 0
	metricsMap["level_1_bytes"] = 0
	metricsMap["level_2_files"] = 0
	metricsMap["level_2_bytes"] = 0

	for level, files := range levelsSnapshot {
		keyFiles := fmt.Sprintf("level_%d_files", level)
		keyBytes := fmt.Sprintf("level_%d_bytes", level)

		metricsMap[keyFiles] = int64(len(files))

		var totalBytes int64
		for _, f := range files {
			totalBytes += f.FileSize
		}
		metricsMap[keyBytes] = totalBytes
	}
	// --- KẾT THÚC MÃ MỚI ---

	return metricsMap
}
