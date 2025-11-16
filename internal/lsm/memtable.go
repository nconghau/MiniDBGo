package lsm

import (
	"sync"
	"sync/atomic"

	"github.com/huandu/skiplist"
	// --- MỚI: Import engine ---
	"github.com/nconghau/MiniDBGo/internal/engine"
)

// --- SỬA ĐỔI: Xóa định nghĩa Item (đã ở engine.go) ---

type MemTable struct {
	sl       *skiplist.SkipList
	byteSize int64
	mu       sync.RWMutex
}

// (NewMemTable giữ nguyên)
func NewMemTable() *MemTable {
	return &MemTable{
		sl:       skiplist.New(skiplist.String),
		byteSize: 0,
	}
}

func (m *MemTable) Put(key string, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.sl.GetValue(key); ok { // [cite: 80-81]
		if existingItem, ok := existing.(*engine.Item); ok { // --- SỬA ĐỔI: Dùng engine.Item ---
			atomic.AddInt64(&m.byteSize, -int64(len(existingItem.Value)))
		}
	}

	item := &engine.Item{Value: value, Tombstone: false} // --- SỬA ĐỔI: Dùng engine.Item ---
	m.sl.Set(key, item)
	atomic.AddInt64(&m.byteSize, int64(len(key)+len(value)+16))
}

func (m *MemTable) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.sl.GetValue(key); ok { // [cite: 81-82]
		if existingItem, ok := existing.(*engine.Item); ok { // --- SỬA ĐỔI: Dùng engine.Item ---
			atomic.AddInt64(&m.byteSize, -int64(len(existingItem.Value)))
		}
	}

	item := &engine.Item{Tombstone: true} // --- SỬA ĐỔI: Dùng engine.Item ---
	m.sl.Set(key, item)
	atomic.AddInt64(&m.byteSize, int64(len(key)+8))
}

// --- SỬA ĐỔI: Trả về engine.Item ---
func (m *MemTable) Get(key string) (*engine.Item, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, ok := m.sl.GetValue(key)
	if !ok || val == nil { // [cite: 82-83]
		return nil, false
	}
	return val.(*engine.Item), true
}

func (m *MemTable) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]string, 0, m.sl.Len())
	for el := m.sl.Front(); el != nil; el = el.Next() {
		out = append(out, el.Key().(string))
	}
	return out
}

func (m *MemTable) Size() int64 {
	return int64(m.sl.Len())
}

// ByteSize returns approximate memory usage in bytes
func (m *MemTable) ByteSize() int64 {
	return atomic.LoadInt64(&m.byteSize)
}

// --- SỬA ĐỔI: Trả về map[string]*engine.Item ---
func (m *MemTable) SnapshotAndReset() map[string]*engine.Item {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make(map[string]*engine.Item, m.sl.Len())

	for el := m.sl.Front(); el != nil; el = el.Next() { // [cite: 84-85]
		k := el.Key().(string)
		v := el.Value.(*engine.Item) // --- SỬA ĐỔI ---

		itemCopy := &engine.Item{ // --- SỬA ĐỔI: Dùng engine.Item ---
			Value:     append([]byte(nil), v.Value...),
			Tombstone: v.Tombstone,
		}
		items[k] = itemCopy
	}

	m.sl = skiplist.New(skiplist.String)
	atomic.StoreInt64(&m.byteSize, 0)
	return items
}

// Clear removes all entries (used for testing)
func (m *MemTable) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sl = skiplist.New(skiplist.String)
	atomic.StoreInt64(&m.byteSize, 0)
}

// --- SỬA ĐỔI: Dùng engine.Item ---
func (m *MemTable) Iterate(fn func(key string, item *engine.Item) error) error {
	// ... (logic [cite: 85-87] giữ nguyên, chỉ thay kiểu *Item) ...
	m.mu.RLock()
	defer m.mu.RUnlock()
	for el := m.sl.Front(); el != nil; el = el.Next() {
		k := el.Key().(string)
		v := el.Value.(*engine.Item)
		if err := fn(k, v); err != nil {
			return err
		}
	}
	return nil
}

// --- SỬA ĐỔI: Dùng engine.Item ---
func (m *MemTable) Stats() map[string]interface{} {
	// ... (logic [cite: 87-88] giữ nguyên, chỉ thay kiểu *Item) ...
	m.mu.RLock()
	defer m.mu.RUnlock()
	tombstoneCount := 0
	valueCount := 0
	for el := m.sl.Front(); el != nil; el = el.Next() {
		v := el.Value.(*engine.Item)
		if v.Tombstone {
			tombstoneCount++
		} else {
			valueCount++
		}
	}
	return map[string]interface{}{
		"total_entries":   m.sl.Len(),
		"value_entries":   valueCount,
		"tombstone_count": tombstoneCount,
		"byte_size":       atomic.LoadInt64(&m.byteSize),
	}
}
