package lsm

import (
	// --- MỚI: Import engine ---
	"github.com/nconghau/MiniDBGo/internal/engine"
)

// --- MỚI: Kiểm tra static ---
var _ engine.Batch = (*lsmBatch)(nil)

// --- SỬA ĐỔI: Đổi tên (nội bộ) ---
type batchEntry struct {
	Key       []byte
	Value     []byte
	Tombstone bool
}

// --- SỬA ĐỔI: Đổi tên (nội bộ) ---
type lsmBatch struct {
	entries []*batchEntry
}

// NewBatch (Hàm nội bộ)
func NewBatch() *lsmBatch {
	return &lsmBatch{
		entries: make([]*batchEntry, 0, 10),
	}
}

// Put triển khai engine.Batch
func (b *lsmBatch) Put(key, value []byte) {
	b.entries = append(b.entries, &batchEntry{
		Key:       key,
		Value:     value, // [cite: 161-162]
		Tombstone: false,
	})
}

// Delete triển khai engine.Batch
func (b *lsmBatch) Delete(key []byte) {
	b.entries = append(b.entries, &batchEntry{
		Key:       key,
		Value:     nil,
		Tombstone: true,
	})
}

// Size triển khai engine.Batch
func (b *lsmBatch) Size() int {
	return len(b.entries)
}
