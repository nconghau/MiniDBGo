package lsm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/huandu/skiplist"
)

// Iterator là một interface (hợp đồng) chung cho tất cả các trình lặp
type Iterator interface {
	// Next di chuyển con trỏ đến mục tiếp theo.
	// Trả về false nếu hết dữ liệu hoặc có lỗi.
	Next() bool
	// Key trả về key của mục hiện tại (sau khi gọi Next()).
	Key() string
	// Value trả về Item (value + tombstone) của mục hiện tại.
	Value() *Item
	// Close giải phóng tài nguyên.
	Close() error
	// Error trả về lỗi (nếu có) đã xảy ra.
	Error() error
}

// --- memTableIterator ---
// Lặp qua skiplist trong bộ nhớ (đã được RLock)

type memTableIterator struct {
	node  *skiplist.Element // Con trỏ đến mục *hiện tại*
	mem   *MemTable         // Giữ tham chiếu để RUnlock
	key   string
	value *Item
}

// NewMemTableIterator tạo một iterator cho MemTable
// Nắm giữ RLock của MemTable cho đến khi Close()
func NewMemTableIterator(mem *MemTable) Iterator {
	mem.mu.RLock()
	return &memTableIterator{
		node: mem.sl.Front(), // Bắt đầu tại mục đầu tiên
		mem:  mem,
	}
}

func (it *memTableIterator) Next() bool {
	if it.node == nil {
		return false
	}

	// Lấy giá trị hiện tại
	it.key = it.node.Key().(string)
	it.value = it.node.Value.(*Item)

	// Di chuyển con trỏ
	it.node = it.node.Next()

	return true
}

func (it *memTableIterator) Key() string {
	return it.key
}

func (it *memTableIterator) Value() *Item {
	return it.value
}

func (it *memTableIterator) Close() error {
	it.mem.mu.RUnlock()
	it.node = nil
	it.mem = nil
	return nil
}

func (it *memTableIterator) Error() error {
	return nil
}

// --- blockIterator ---
// Lặp qua các entry *bên trong* một khối dữ liệu (data block)
// Đây là iterator nội bộ, không cần export

type blockIterator struct {
	r     *bytes.Reader
	key   string
	value *Item
	err   error
}

func newBlockIterator(blockData []byte) *blockIterator {
	return &blockIterator{
		r: bytes.NewReader(blockData),
	}
}

func (it *blockIterator) Next() bool {
	if it.r.Len() == 0 {
		return false
	}

	var klen, vlen uint32
	var flag byte
	var err error

	if err = binary.Read(it.r, binary.LittleEndian, &klen); err != nil {
		if err == io.EOF {
			return false
		}
		it.err = fmt.Errorf("read data keylen: %w", err)
		return false
	}
	if err = binary.Read(it.r, binary.LittleEndian, &vlen); err != nil {
		it.err = fmt.Errorf("read data vallen: %w", err)
		return false
	}
	flag, err = it.r.ReadByte()
	if err != nil {
		it.err = fmt.Errorf("read data flag: %w", err)
		return false
	}

	kb := make([]byte, klen)
	if _, err = io.ReadFull(it.r, kb); err != nil {
		it.err = fmt.Errorf("read data key: %w", err)
		return false
	}

	vb := make([]byte, vlen)
	if vlen > 0 {
		if _, err = io.ReadFull(it.r, vb); err != nil {
			it.err = fmt.Errorf("read data value: %w", err)
			return false
		}
	}

	it.key = string(kb)
	it.value = &Item{
		Value:     vb,
		Tombstone: flag == 1,
	}
	return true
}

func (it *blockIterator) Key() string  { return it.key }
func (it *blockIterator) Value() *Item { return it.value }
func (it *blockIterator) Error() error { return it.err }
func (it *blockIterator) Close() error { return nil } // Không làm gì

// --- sstIterator ---
// Lặp qua tất cả các khối (block) trong một tệp SSTable

type sstIterator struct {
	f     *os.File
	index []blockIndexEntry // Index Block (đọc 1 lần)

	blockIdx  int            // Chỉ số khối (data block) hiện tại
	blockIter *blockIterator // Iterator cho khối hiện tại

	key   string
	value *Item
	err   error
}

// NewSSTableIterator tạo một iterator cho một tệp SSTable
// Sử dụng logic từ Giai đoạn 1 (Block Index) để tải index
func NewSSTableIterator(path string) (Iterator, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	// 1. Đọc Footer (để lấy vị trí Index Block)
	if stat.Size() < (8 + SSTFooterSize) {
		f.Close()
		return nil, fmt.Errorf("file too small or corrupt")
	}
	footerData := make([]byte, SSTFooterSize)
	if _, err := f.ReadAt(footerData, stat.Size()-SSTFooterSize); err != nil {
		f.Close()
		return nil, fmt.Errorf("read footer: %w", err)
	}

	var indexOffset, indexLen uint64
	// Chúng ta chỉ cần indexOffset và indexLen
	r := bytes.NewReader(footerData)
	binary.Read(r, binary.LittleEndian, &indexOffset)
	binary.Read(r, binary.LittleEndian, &indexLen)
	// (Bỏ qua phần còn lại của footer vì iterator không cần bloom)

	// 2. Đọc toàn bộ Index Block vào bộ nhớ
	indexData := make([]byte, indexLen)
	if _, err := f.ReadAt(indexData, int64(indexOffset)); err != nil {
		f.Close()
		return nil, fmt.Errorf("read index block: %w", err)
	}

	idxReader := bytes.NewReader(indexData)
	var numEntries uint32
	if err := binary.Read(idxReader, binary.LittleEndian, &numEntries); err != nil {
		f.Close()
		return nil, fmt.Errorf("read index entry count: %w", err)
	}

	indexEntries := make([]blockIndexEntry, numEntries)
	for i := 0; i < int(numEntries); i++ {
		var klen uint32
		if err := binary.Read(idxReader, binary.LittleEndian, &klen); err != nil {
			f.Close()
			return nil, fmt.Errorf("read index entry klen: %w", err)
		}
		keyBytes := make([]byte, klen)
		if _, err := io.ReadFull(idxReader, keyBytes); err != nil {
			f.Close()
			return nil, fmt.Errorf("read index entry key: %w", err)
		}
		indexEntries[i].lastKey = string(keyBytes)
		if err := binary.Read(idxReader, binary.LittleEndian, &indexEntries[i].offset); err != nil {
			f.Close()
			return nil, fmt.Errorf("read index entry offset: %w", err)
		}
		if err := binary.Read(idxReader, binary.LittleEndian, &indexEntries[i].length); err != nil {
			f.Close()
			return nil, fmt.Errorf("read index entry length: %w", err)
		}
	}

	it := &sstIterator{
		f:        f,
		index:    indexEntries,
		blockIdx: -1, // Sẽ được +1 khi loadNextBlock
	}

	return it, nil
}

// loadNextBlock tải khối tiếp theo từ SSTable
func (it *sstIterator) loadNextBlock() bool {
	it.blockIdx++
	if it.blockIdx >= len(it.index) {
		return false // Hết khối
	}

	entry := it.index[it.blockIdx]

	dataBlock := make([]byte, entry.length)
	if _, err := it.f.ReadAt(dataBlock, entry.offset); err != nil {
		it.err = fmt.Errorf("read data block: %w", err)
		return false
	}

	it.blockIter = newBlockIterator(dataBlock)
	return true
}

func (it *sstIterator) Next() bool {
	for {
		if it.blockIter == nil {
			// Khối đầu tiên, hoặc khối trước đã hết
			if !it.loadNextBlock() {
				return false // Hết khối, hết sst
			}
		}

		if it.blockIter.Next() {
			// Tìm thấy entry trong khối
			it.key = it.blockIter.Key()
			it.value = it.blockIter.Value()
			return true
		}

		if it.blockIter.Error() != nil {
			it.err = it.blockIter.Error()
			return false
		}

		// Hết khối hiện tại, vòng lặp for sẽ chạy lại
		// và loadNextBlock()
		it.blockIter = nil
	}
}

func (it *sstIterator) Key() string {
	return it.key
}

func (it *sstIterator) Value() *Item {
	return it.value
}

func (it *sstIterator) Close() error {
	it.blockIter = nil
	it.index = nil
	return it.f.Close()
}

func (it *sstIterator) Error() error {
	return it.err
}
