package lsm

import (
	"fmt"
	"hash/fnv"
)

// BloomFilter được tối ưu hóa sử dụng bitset (slice of bytes)
type BloomFilter struct {
	bits []byte
	k    int    // Số lượng hàm hash
	n    uint32 // Số lượng bit
}

// NewBloomFilter tạo một bloom filter với n bits và k hàm hash
func NewBloomFilter(numBits uint32, numHashes int) *BloomFilter {
	if numBits == 0 {
		numBits = 1 // Tránh lỗi chia cho 0
	}
	// Tính toán số byte cần thiết
	m := (numBits + 7) / 8
	return &BloomFilter{
		bits: make([]byte, m),
		k:    numHashes,
		n:    numBits,
	}
}

// hash tính toán giá trị hash thứ i cho key
func (bf *BloomFilter) hash(i int, key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(fmt.Sprintf("%d%s", i, key)))
	// Modulo cho số lượng bit (n), không phải số lượng byte
	return h.Sum32() % bf.n
}

// Add thêm một key vào bộ lọc
func (bf *BloomFilter) Add(key string) {
	for i := 0; i < bf.k; i++ {
		pos := bf.hash(i, key)
		// Đặt bit tại vị trí pos
		bf.bits[pos/8] |= (1 << (pos % 8))
	}
}

// MightContain kiểm tra xem key có thể có trong bộ lọc hay không
func (bf *BloomFilter) MightContain(key string) bool {
	for i := 0; i < bf.k; i++ {
		pos := bf.hash(i, key)
		// Kiểm tra xem bit tại vị trí pos có được đặt hay không
		if (bf.bits[pos/8] & (1 << (pos % 8))) == 0 {
			return false
		}
	}
	return true
}

// ToBytes trả về dữ liệu thô (bitset) của bộ lọc
func (bf *BloomFilter) ToBytes() []byte {
	return bf.bits
}

// NewFromBytes tạo lại một BloomFilter từ dữ liệu thô và các tham số
func NewFromBytes(data []byte, numBits uint32, numHashes int) *BloomFilter {
	return &BloomFilter{
		bits: data,
		k:    numHashes,
		n:    numBits,
	}
}
