package lsm

// BatchEntry đại diện cho một thao tác (Put hoặc Delete) trong một Batch
type BatchEntry struct {
	Key       []byte
	Value     []byte
	Tombstone bool
}

// Batch là một tập hợp các thao tác ghi được thực hiện nguyên tử
type Batch struct {
	entries []*BatchEntry
}

// NewBatch tạo một batch mới
func NewBatch() *Batch {
	return &Batch{
		entries: make([]*BatchEntry, 0, 10), // Khởi tạo với capacity nhỏ
	}
}

// Put thêm một thao tác Set/Put vào batch
func (b *Batch) Put(key, value []byte) {
	b.entries = append(b.entries, &BatchEntry{
		Key:       key,
		Value:     value,
		Tombstone: false,
	})
}

// Delete thêm một thao tác Xóa vào batch
func (b *Batch) Delete(key []byte) {
	b.entries = append(b.entries, &BatchEntry{
		Key:       key,
		Value:     nil, // Giá trị là nil đối với tombstone
		Tombstone: true,
	})
}

// Size trả về số lượng thao tác trong batch
func (b *Batch) Size() int {
	return len(b.entries)
}
