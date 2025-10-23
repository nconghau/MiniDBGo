package lsm

import (
	"fmt"
	"hash/fnv"
)

type BloomFilter struct {
	bits []bool
	k    int
	n    uint32
}

func NewBloomFilter(n uint32, k int) *BloomFilter {
	return &BloomFilter{
		bits: make([]bool, n),
		k:  	  k,
		n:    n,
	}
}

func (bf *BloomFilter) hash(i int, key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(fmt.Sprintf("%d%s", i, key)))
	return h.Sum32() % bf.n
}

func (bf *BloomFilter) Add(key string) {
	for i := 0; i < bf.k; i++ {
		pos := bf.hash(i, key)
		bf.bits[pos] = true
	}
}

func (bf *BloomFilter) MightContain(key string) bool {
	for i := 0; i < bf.k; i++ {
		pos := bf.hash(i, key)
		if !bf.bits[pos] {
			return false
		}
	}
	return true
}
