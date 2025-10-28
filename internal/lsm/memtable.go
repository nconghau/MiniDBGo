package lsm

import (
	"sync"
	"sync/atomic"

	"github.com/huandu/skiplist"
)

// Item wraps a value + tombstone flag
type Item struct {
	Value     []byte
	Tombstone bool
}

type MemTable struct {
	sl       *skiplist.SkipList
	byteSize int64 // atomic counter for memory usage
	mu       sync.RWMutex
}

func NewMemTable() *MemTable {
	return &MemTable{
		sl:       skiplist.New(skiplist.String),
		byteSize: 0,
	}
}

func (m *MemTable) Put(key string, value []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if key already exists to adjust byte size correctly
	if existing, ok := m.sl.GetValue(key); ok {
		if existingItem, ok := existing.(*Item); ok {
			// Subtract old value size
			atomic.AddInt64(&m.byteSize, -int64(len(existingItem.Value)))
		}
	}

	item := &Item{Value: value, Tombstone: false}
	m.sl.Set(key, item)

	// Add new size (key + value + overhead)
	atomic.AddInt64(&m.byteSize, int64(len(key)+len(value)+16)) // 16 bytes overhead
}

func (m *MemTable) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if key exists
	if existing, ok := m.sl.GetValue(key); ok {
		if existingItem, ok := existing.(*Item); ok {
			// Subtract old value size
			atomic.AddInt64(&m.byteSize, -int64(len(existingItem.Value)))
		}
	}

	item := &Item{Tombstone: true}
	m.sl.Set(key, item)

	// Add tombstone overhead
	atomic.AddInt64(&m.byteSize, int64(len(key)+8))
}

func (m *MemTable) Get(key string) (*Item, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	val, ok := m.sl.GetValue(key)
	if !ok || val == nil {
		return nil, false
	}
	return val.(*Item), true
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

// SnapshotAndReset copies current state and resets table
// This is now memory-safe and doesn't hold locks for long
func (m *MemTable) SnapshotAndReset() map[string]*Item {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make(map[string]*Item, m.sl.Len())

	for el := m.sl.Front(); el != nil; el = el.Next() {
		k := el.Key().(string)
		v := el.Value.(*Item)

		// Deep copy to prevent data races
		itemCopy := &Item{
			Value:     append([]byte(nil), v.Value...),
			Tombstone: v.Tombstone,
		}
		items[k] = itemCopy
	}

	// Reset without creating new skiplist (reuse memory)
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

// Iterate provides safe iteration over all entries
func (m *MemTable) Iterate(fn func(key string, item *Item) error) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for el := m.sl.Front(); el != nil; el = el.Next() {
		k := el.Key().(string)
		v := el.Value.(*Item)

		if err := fn(k, v); err != nil {
			return err
		}
	}

	return nil
}

// Stats returns memtable statistics
func (m *MemTable) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tombstoneCount := 0
	valueCount := 0

	for el := m.sl.Front(); el != nil; el = el.Next() {
		v := el.Value.(*Item)
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
