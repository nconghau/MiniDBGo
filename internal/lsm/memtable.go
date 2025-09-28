package lsm

import (
	"github.com/huandu/skiplist"
)

// Item wraps a value + tombstone flag
type Item struct {
	Value     []byte
	Tombstone bool
}

type MemTable struct {
	sl *skiplist.SkipList
}

func NewMemTable() *MemTable {
	return &MemTable{
		sl: skiplist.New(skiplist.String),
	}
}

func (m *MemTable) Put(key string, value []byte) {
	m.sl.Set(key, &Item{Value: value, Tombstone: false})
}

func (m *MemTable) Delete(key string) {
	m.sl.Set(key, &Item{Tombstone: true})
}

func (m *MemTable) Get(key string) (*Item, bool) {
	val, ok := m.sl.GetValue(key)
	if !ok || val == nil {
		return nil, false
	}
	return val.(*Item), true
}

func (m *MemTable) Keys() []string {
	out := []string{}
	for el := m.sl.Front(); el != nil; el = el.Next() {
		out = append(out, el.Key().(string))
	}
	return out
}

func (m *MemTable) Size() int64 {
	return int64(m.sl.Len())
}

// SnapshotAndReset copies current state and resets table
func (m *MemTable) SnapshotAndReset() map[string]*Item {
	items := map[string]*Item{}
	for el := m.sl.Front(); el != nil; el = el.Next() {
		k := el.Key().(string)
		v := el.Value.(*Item)
		items[k] = v
	}
	m.sl = skiplist.New(skiplist.String)
	return items
}
