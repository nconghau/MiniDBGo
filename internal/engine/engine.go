package engine

// (Không import lsm)

// --- MỚI: Di chuyển Item (từ memtable.go) sang đây ---
type Item struct {
	Value     []byte
	Tombstone bool
}

// --- MỚI: Định nghĩa Iterator interface (từ iterator.go) ---
type Iterator interface {
	Next() bool
	Key() string
	Value() *Item // Sử dụng engine.Item
	Close() error
	Error() error
}

// --- MỚI: Định nghĩa Batch interface ---
type Batch interface {
	Put(key, value []byte)
	Delete(key []byte)
	Size() int
}

// DB Engine interface
// --- SỬA ĐỔI: Sử dụng các interface cục bộ ---
type Engine interface {
	Put(key, value []byte) error
	Update(key, value []byte) error
	Delete(key []byte) error
	Get(key []byte) ([]byte, error)
	DumpDB(path string) error
	RestoreDB(path string) error
	Compact() error
	Close() error
	GetMetrics() map[string]int64
	IterKeysWithLimit(limit int) ([]string, error)

	NewBatch() Batch                // Trả về interface
	ApplyBatch(b Batch) error       // Chấp nhận interface
	NewIterator() (Iterator, error) // Trả về interface
}

// --- SỬA ĐỔI: Xóa hàm Open() ---
// (Hàm Open() không thể ở đây vì nó tạo ra
// phụ thuộc vào lsm. Chúng ta sẽ gọi lsm.OpenLSM trực tiếp
// từ main.go)
