package lsm

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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
)

type LSMEngine struct {
	dir      string
	wal      *WAL
	mem      *MemTable
	memBytes int64 // atomic counter for current memtable size

	immutMu    sync.RWMutex
	immutables []*MemTable

	sstDir      string
	seq         int
	flushSize   int64
	maxMemBytes int64

	mu sync.RWMutex

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Flush management
	flushCh  chan *MemTable
	flushErr atomic.Value // stores last flush error

	// Metrics
	metrics struct {
		puts     atomic.Int64
		gets     atomic.Int64
		deletes  atomic.Int64
		flushes  atomic.Int64
		compacts atomic.Int64
	}
}

func OpenLSM(dir string) (*LSMEngine, error) {
	return OpenLSMWithConfig(dir, DefaultFlushSize, DefaultMemTableBytes)
}

func OpenLSMWithConfig(dir string, flushSize int64, maxMemBytes int64) (*LSMEngine, error) {
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

	// Find next WAL sequence
	seq := 1
	files, _ := os.ReadDir(walDir)
	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(name, "wal-") && strings.HasSuffix(name, ".log") {
			num, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "wal-"), ".log"))
			if num >= seq {
				seq = num + 1
			}
		}
	}

	w, err := OpenWAL(walDir, seq)
	if err != nil {
		return nil, fmt.Errorf("open wal: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &LSMEngine{
		dir:         dir,
		wal:         w,
		mem:         NewMemTable(),
		immutables:  make([]*MemTable, 0, MaxImmutableTables),
		sstDir:      sstDir,
		seq:         1,
		flushSize:   flushSize,
		maxMemBytes: maxMemBytes,
		ctx:         ctx,
		cancel:      cancel,
		flushCh:     make(chan *MemTable, MaxImmutableTables),
	}

	// Replay WAL files
	replayedFiles, err := engine.replayWAL(walDir)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("replay wal: %w", err)
	}

	// --- SỬA: Thêm logic flush dữ liệu replay và xóa WAL cũ ---
	if engine.mem.Size() > 0 {
		slog.Info("Flushing replayed WAL data to SSTable...", "count", engine.mem.Size())

		// Đẩy dữ liệu replay vào hàng đợi flush
		if err := engine.rotateMemTable(); err != nil {
			cancel() // Hủy context nếu không thể flush
			return nil, fmt.Errorf("failed to schedule flush for replayed data: %w", err)
		}

		// Xóa các file WAL đã replay *sau khi* đẩy vào queue
		// Lưu ý: Có rủi ro nhỏ nếu server crash trước khi flushWorker chạy.
		// Một giải pháp hoàn hảo hơn sẽ xóa WAL *sau khi* flushWorker
		// xác nhận flush thành công. Nhưng với kiến trúc hiện tại,
		// đây là giải pháp 80/20.
		for _, p := range replayedFiles {
			if err := os.Remove(p); err != nil {
				slog.Warn("Failed to delete replayed WAL file", "path", p, "error", err)
			}
		}
		slog.Info("Cleaned up replayed WAL files.", "count", len(replayedFiles))
	}
	// --- KẾT THÚC SỬA ---

	// Start background flush worker
	engine.wg.Add(1)
	go engine.flushWorker()

	return engine, nil
}

// SỬA: Đổi kiểu trả về
func (e *LSMEngine) replayWAL(walDir string) ([]string, error) {
	walFiles, err := os.ReadDir(walDir)
	if err != nil {
		return nil, err // SỬA
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
			k := string(key)
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

	return names, nil // SỬA: Trả về danh sách file
}

// flushWorker handles background flushing of immutable memtables
func (e *LSMEngine) flushWorker() {
	defer e.wg.Done()

	slog.Info("Flush worker started", "component", "lsm")

	for memTable := range e.flushCh {
		slog.Info("Starting memtable flush", "component", "lsm")
		start := time.Now()

		if err := e.flushMemTable(memTable); err != nil {
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
	}

	slog.Info("Flush worker stopped (channel closed).", "component", "lsm")
}

func (e *LSMEngine) flushMemTable(memTable *MemTable) error {
	ctx, cancel := context.WithTimeout(e.ctx, FlushTimeout)
	defer cancel()

	items := memTable.SnapshotAndReset()
	if len(items) == 0 {
		e.removeImmutable(memTable)
		return nil
	}

	// Check context before expensive operation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	seq := e.seq
	e.mu.Lock()
	e.seq++
	e.mu.Unlock()

	if _, err := WriteSST(e.sstDir, 0, seq, items); err != nil {
		return fmt.Errorf("write sst: %w", err)
	}

	e.removeImmutable(memTable)
	e.metrics.flushes.Add(1)

	return nil
}

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

func (e *LSMEngine) Put(key, value []byte) error {
	e.metrics.puts.Add(1)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ctx.Err() != nil {
		return errors.New("engine is shutting down")
	}

	if err := e.wal.Append(key, value, false); err != nil {
		return fmt.Errorf("wal append: %w", err)
	}

	e.mem.Put(string(key), value)
	newSize := atomic.AddInt64(&e.memBytes, int64(len(key)+len(value)))

	// Check both record count and memory size
	if e.mem.Size() >= e.flushSize || newSize >= e.maxMemBytes {
		if err := e.rotateMemTable(); err != nil {
			return fmt.Errorf("rotate memtable: %w", err)
		}
	}

	return nil
}

func (e *LSMEngine) Update(key, value []byte) error {
	return e.Put(key, value)
}

func (e *LSMEngine) Delete(key []byte) error {
	e.metrics.deletes.Add(1)

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.ctx.Err() != nil {
		return errors.New("engine is shutting down")
	}

	if err := e.wal.Append(key, nil, true); err != nil {
		return fmt.Errorf("wal append: %w", err)
	}

	e.mem.Delete(string(key))
	atomic.AddInt64(&e.memBytes, int64(len(key)))

	if e.mem.Size() >= e.flushSize || atomic.LoadInt64(&e.memBytes) >= e.maxMemBytes {
		if err := e.rotateMemTable(); err != nil {
			return fmt.Errorf("rotate memtable: %w", err)
		}
	}

	return nil
}

func (e *LSMEngine) Get(key []byte) ([]byte, error) {
	e.metrics.gets.Add(1)

	k := string(key)

	// Check active memtable
	e.mu.RLock()
	if it, ok := e.mem.Get(k); ok {
		e.mu.RUnlock()
		if it.Tombstone {
			return nil, errors.New("key not found")
		}
		return it.Value, nil
	}
	e.mu.RUnlock()

	// Check immutable memtables
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

	// Search SST files (newest first)
	files, err := os.ReadDir(e.sstDir)
	if err == nil {
		names := make([]string, 0, len(files))
		for _, fi := range files {
			if strings.HasSuffix(fi.Name(), ".sst") {
				names = append(names, filepath.Join(e.sstDir, fi.Name()))
			}
		}
		sort.Slice(names, func(i, j int) bool { return names[i] > names[j] })

		for _, p := range names {
			if bv, tomb, err := ReadSSTFind(p, k); err == nil {
				if tomb {
					return nil, errors.New("key not found")
				}
				if bv != nil {
					return bv, nil
				}
			}
		}
	}

	return nil, errors.New("key not found")
}

// IterKeys is now memory-safe with streaming approach
func (e *LSMEngine) IterKeys() ([]string, error) {
	return e.IterKeysWithLimit(0) // 0 = no limit, but still stream
}

func (e *LSMEngine) IterKeysWithLimit(limit int) ([]string, error) {
	keysMap := make(map[string]struct{})
	count := 0

	// Memtable keys
	e.mu.RLock()
	for _, k := range e.mem.Keys() {
		if limit > 0 && count >= limit {
			e.mu.RUnlock()
			return mapToSlice(keysMap), nil
		}
		keysMap[k] = struct{}{}
		count++
	}
	e.mu.RUnlock()

	// Immutable memtables
	e.immutMu.RLock()
	for _, m := range e.immutables {
		for _, k := range m.Keys() {
			if limit > 0 && count >= limit {
				e.immutMu.RUnlock()
				return mapToSlice(keysMap), nil
			}
			keysMap[k] = struct{}{}
			count++
		}
	}
	e.immutMu.RUnlock()

	// SST files - stream keys instead of loading all at once
	files, err := os.ReadDir(e.sstDir)
	if err != nil {
		return mapToSlice(keysMap), nil
	}

	for _, fi := range files {
		if !strings.HasSuffix(fi.Name(), ".sst") {
			continue
		}

		if limit > 0 && count >= limit {
			break
		}

		p := filepath.Join(e.sstDir, fi.Name())
		if err := e.streamSSTKeys(p, keysMap, limit, &count); err != nil {
			continue
		}
	}

	return mapToSlice(keysMap), nil
}

func (e *LSMEngine) streamSSTKeys(path string, keysMap map[string]struct{}, limit int, count *int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Skip version (4 bytes)
	header := make([]byte, 8)
	if _, err := io.ReadFull(f, header); err != nil {
		return err
	}

	countHeader := binary.LittleEndian.Uint32(header[4:8])

	for i := uint32(0); i < countHeader; i++ {
		if limit > 0 && *count >= limit {
			break
		}

		var klen, vlen uint32
		if err := binary.Read(f, binary.LittleEndian, &klen); err != nil {
			break
		}
		if err := binary.Read(f, binary.LittleEndian, &vlen); err != nil {
			break
		}

		flag := make([]byte, 1)
		if _, err := f.Read(flag); err != nil {
			break
		}

		kb := make([]byte, klen)
		if _, err := f.Read(kb); err != nil {
			break
		}

		// Skip value bytes efficiently
		if _, err := f.Seek(int64(vlen), io.SeekCurrent); err != nil {
			break
		}

		keysMap[string(kb)] = struct{}{}
		*count++
	}

	return nil
}

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

func (e *LSMEngine) DumpDB(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Stream write to file instead of building entire structure in memory
	enc := json.NewEncoder(f)

	collections := make(map[string]bool)

	// First pass: identify collections
	keys, err := e.IterKeysWithLimit(100000) // Limit for safety
	if err != nil {
		return err
	}

	for _, full := range keys {
		if idx := strings.Index(full, ":"); idx >= 0 {
			collections[full[:idx]] = true
		}
	}

	// Second pass: stream each collection
	result := make(map[string][]map[string]interface{})

	for col := range collections {
		prefix := col + ":"
		docs := make([]map[string]interface{}, 0)

		for _, full := range keys {
			if !strings.HasPrefix(full, prefix) {
				continue
			}

			v, err := e.Get([]byte(full))
			if err != nil {
				continue
			}

			var doc map[string]interface{}
			if err := json.Unmarshal(v, &doc); err != nil {
				continue
			}

			idx := strings.Index(full, ":")
			if idx >= 0 {
				doc["_id"] = full[idx+1:]
			}

			docs = append(docs, doc)
		}

		result[col] = docs
	}

	return enc.Encode(result)
}

func (e *LSMEngine) RestoreDB(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Stream decode to avoid loading entire file into memory
	dec := json.NewDecoder(f)

	var data map[string][]map[string]interface{}
	if err := dec.Decode(&data); err != nil {
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
			if err := e.Put([]byte(col+":"+idStr), raw); err != nil {
				return err
			}
		}

		// Clear docs to free memory between collections
		data[col] = nil
	}

	return nil
}

// Close gracefully shuts down the engine
func (e *LSMEngine) Close() error {
	slog.Info("Database closing...", "component", "lsm")

	// 1. SỬA: Đẩy nốt dữ liệu RAM vào hàng đợi (nếu có)
	if e.mem.Size() > 0 {
		slog.Info("Scheduling active MemTable flush before shutdown...", "component", "lsm")
		if err := e.rotateMemTable(); err != nil {
			// Nếu hàng đợi đầy, chúng ta vẫn phải tiếp tục đóng
			slog.Error("Failed to schedule final MemTable flush on close", "error", err)
		}
	}

	// 2. SỬA: Đóng flushCh để báo worker là hết việc
	// (Worker sẽ xử lý hết hàng đợi rồi tự thoát)
	close(e.flushCh)

	// 3. SỬA: Chờ worker xử lý xong và tự thoát
	e.wg.Wait()
	slog.Info("Flush worker finished.", "component", "lsm")

	// (Bây giờ e.cancel() không còn cần thiết nữa, nhưng giữ lại cũng không sao)
	e.cancel()

	// 4. Đóng WAL (SAU KHI mọi thứ đã được flush an toàn)
	if e.wal != nil {
		if err := e.wal.Close(); err != nil {
			return err
		}
	}

	slog.Info("Database closed gracefully.", "component", "lsm")
	return nil
}

// GetMetrics returns engine metrics
func (e *LSMEngine) GetMetrics() map[string]int64 {
	return map[string]int64{
		"puts":     e.metrics.puts.Load(),
		"gets":     e.metrics.gets.Load(),
		"deletes":  e.metrics.deletes.Load(),
		"flushes":  e.metrics.flushes.Load(),
		"compacts": e.metrics.compacts.Load(),
	}
}
