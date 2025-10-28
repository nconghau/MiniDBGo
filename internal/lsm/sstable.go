package lsm

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const (
	// SSTable format version
	SSTVersion = 1

	// Buffer sizes
	SSTWriteBufferSize = 256 * 1024 // 256KB
	SSTReadBufferSize  = 128 * 1024 // 128KB

	// SSTable file format:
	// [Header: 8 bytes] [Entries...] [Index: variable] [Footer: 16 bytes]
	// Header: version(4) + count(4)
	// Entry: keyLen(4) + valueLen(4) + flag(1) + key + value
	// Footer: indexOffset(8) + indexSize(8)
)

// SSTMetadata holds SSTable file metadata
type SSTMetadata struct {
	Path        string
	Level       int
	Sequence    int
	KeyCount    uint32
	MinKey      string
	MaxKey      string
	FileSize    int64
	BloomFilter *BloomFilter
}

// SSTWriter handles writing SSTable files
type SSTWriter struct {
	file   *os.File
	writer *bufio.Writer
	path   string
	count  uint32
	minKey string
	maxKey string
	bloom  *BloomFilter
}

// NewSSTWriter creates a new SSTable writer
func NewSSTWriter(path string, estimatedKeys uint32) (*SSTWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create sst file: %w", err)
	}

	w := &SSTWriter{
		file:   f,
		writer: bufio.NewWriterSize(f, SSTWriteBufferSize),
		path:   path,
		count:  0,
		bloom:  NewBloomFilter(estimatedKeys*10, 3), // 10 bits per key, 3 hash functions
	}

	// Write header placeholder (will be updated on close)
	header := make([]byte, 8)
	binary.LittleEndian.PutUint32(header[0:4], SSTVersion)
	binary.LittleEndian.PutUint32(header[4:8], 0) // count placeholder
	if _, err := w.writer.Write(header); err != nil {
		f.Close()
		return nil, fmt.Errorf("write header: %w", err)
	}

	return w, nil
}

// WriteEntry writes a single key-value entry
func (w *SSTWriter) WriteEntry(key string, item *Item) error {
	if w.count == 0 {
		w.minKey = key
	}
	w.maxKey = key
	w.count++

	// Add to bloom filter
	w.bloom.Add(key)

	kb := []byte(key)
	vb := item.Value
	if item.Tombstone {
		vb = []byte{} // Empty value for tombstone
	}

	// Write entry: keyLen(4) + valueLen(4) + flag(1) + key + value
	if err := binary.Write(w.writer, binary.LittleEndian, uint32(len(kb))); err != nil {
		return err
	}
	if err := binary.Write(w.writer, binary.LittleEndian, uint32(len(vb))); err != nil {
		return err
	}

	flag := byte(0)
	if item.Tombstone {
		flag = 1
	}
	if _, err := w.writer.Write([]byte{flag}); err != nil {
		return err
	}

	if _, err := w.writer.Write(kb); err != nil {
		return err
	}
	if len(vb) > 0 {
		if _, err := w.writer.Write(vb); err != nil {
			return err
		}
	}

	return nil
}

// Close finalizes the SSTable file
func (w *SSTWriter) Close() error {
	// Flush buffered data
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("flush writer: %w", err)
	}

	// Update header with actual count
	if _, err := w.file.Seek(4, io.SeekStart); err != nil {
		return fmt.Errorf("seek header: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, w.count); err != nil {
		return fmt.Errorf("write count: %w", err)
	}

	// Sync and close
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}

	return w.file.Close()
}

// GetMetadata returns metadata about the written SSTable
func (w *SSTWriter) GetMetadata() *SSTMetadata {
	stat, _ := os.Stat(w.path)
	return &SSTMetadata{
		Path:        w.path,
		KeyCount:    w.count,
		MinKey:      w.minKey,
		MaxKey:      w.maxKey,
		FileSize:    stat.Size(),
		BloomFilter: w.bloom,
	}
}

// WriteSST writes a complete SSTable from a map of items
func WriteSST(dir string, level, seq int, items map[string]*Item) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no items to write")
	}

	// Sort keys for ordered SSTable
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	path := filepath.Join(dir, fmt.Sprintf("sst-L%d-%06d.sst", level, seq))
	writer, err := NewSSTWriter(path, uint32(len(items)))
	if err != nil {
		return "", err
	}

	// Write entries in sorted order
	for _, key := range keys {
		if err := writer.WriteEntry(key, items[key]); err != nil {
			writer.Close()
			os.Remove(path)
			return "", fmt.Errorf("write entry %s: %w", key, err)
		}
	}

	if err := writer.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close writer: %w", err)
	}

	return path, nil
}

// ReadSSTFind searches for a key in an SSTable file
func ReadSSTFind(path string, key string) ([]byte, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, SSTReadBufferSize)

	// Read header
	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, false, fmt.Errorf("read header: %w", err)
	}

	version := binary.LittleEndian.Uint32(header[0:4])
	if version != SSTVersion {
		return nil, false, fmt.Errorf("unsupported version: %d", version)
	}

	count := binary.LittleEndian.Uint32(header[4:8])

	// Sequential scan (could be optimized with binary search using index)
	for i := uint32(0); i < count; i++ {
		var klen, vlen uint32

		if err := binary.Read(reader, binary.LittleEndian, &klen); err != nil {
			return nil, false, fmt.Errorf("read keylen: %w", err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &vlen); err != nil {
			return nil, false, fmt.Errorf("read valuelen: %w", err)
		}

		flag := make([]byte, 1)
		if _, err := reader.Read(flag); err != nil {
			return nil, false, fmt.Errorf("read flag: %w", err)
		}

		kb := make([]byte, klen)
		if _, err := io.ReadFull(reader, kb); err != nil {
			return nil, false, fmt.Errorf("read key: %w", err)
		}

		vb := make([]byte, vlen)
		if vlen > 0 {
			if _, err := io.ReadFull(reader, vb); err != nil {
				return nil, false, fmt.Errorf("read value: %w", err)
			}
		}

		if string(kb) == key {
			if flag[0] == 1 {
				return nil, true, nil // tombstone found
			}
			return vb, false, nil
		}
	}

	return nil, false, os.ErrNotExist
}

// ReadAllSST reads all entries from an SSTable (memory-bounded)
func ReadAllSST(path string) (map[string]*Item, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, SSTReadBufferSize)

	// Read header
	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	version := binary.LittleEndian.Uint32(header[0:4])
	if version != SSTVersion {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	count := binary.LittleEndian.Uint32(header[4:8])
	result := make(map[string]*Item, count)

	// Read all entries
	for i := uint32(0); i < count; i++ {
		var klen, vlen uint32

		if err := binary.Read(reader, binary.LittleEndian, &klen); err != nil {
			return nil, fmt.Errorf("read keylen at entry %d: %w", i, err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &vlen); err != nil {
			return nil, fmt.Errorf("read valuelen at entry %d: %w", i, err)
		}

		flag := make([]byte, 1)
		if _, err := reader.Read(flag); err != nil {
			return nil, fmt.Errorf("read flag at entry %d: %w", i, err)
		}

		kb := make([]byte, klen)
		if _, err := io.ReadFull(reader, kb); err != nil {
			return nil, fmt.Errorf("read key at entry %d: %w", i, err)
		}

		vb := make([]byte, vlen)
		if vlen > 0 {
			if _, err := io.ReadFull(reader, vb); err != nil {
				return nil, fmt.Errorf("read value at entry %d: %w", i, err)
			}
		}

		result[string(kb)] = &Item{
			Value:     vb,
			Tombstone: flag[0] == 1,
		}
	}

	return result, nil
}

// IterateSST iterates over all entries in an SSTable without loading all into memory
func IterateSST(path string, fn func(key string, item *Item) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, SSTReadBufferSize)

	// Read header
	header := make([]byte, 8)
	if _, err := io.ReadFull(reader, header); err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	version := binary.LittleEndian.Uint32(header[0:4])
	if version != SSTVersion {
		return fmt.Errorf("unsupported version: %d", version)
	}

	count := binary.LittleEndian.Uint32(header[4:8])

	// Stream entries
	for i := uint32(0); i < count; i++ {
		var klen, vlen uint32

		if err := binary.Read(reader, binary.LittleEndian, &klen); err != nil {
			return fmt.Errorf("read keylen: %w", err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &vlen); err != nil {
			return fmt.Errorf("read valuelen: %w", err)
		}

		flag := make([]byte, 1)
		if _, err := reader.Read(flag); err != nil {
			return fmt.Errorf("read flag: %w", err)
		}

		kb := make([]byte, klen)
		if _, err := io.ReadFull(reader, kb); err != nil {
			return fmt.Errorf("read key: %w", err)
		}

		vb := make([]byte, vlen)
		if vlen > 0 {
			if _, err := io.ReadFull(reader, vb); err != nil {
				return fmt.Errorf("read value: %w", err)
			}
		}

		item := &Item{
			Value:     vb,
			Tombstone: flag[0] == 1,
		}

		if err := fn(string(kb), item); err != nil {
			return err
		}
	}

	return nil
}

// GetSSTMetadata reads metadata from an SSTable without loading all data
func GetSSTMetadata(path string) (*SSTMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read header
	header := make([]byte, 8)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	version := binary.LittleEndian.Uint32(header[0:4])
	if version != SSTVersion {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	count := binary.LittleEndian.Uint32(header[4:8])

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// Extract level and sequence from filename
	// Format: sst-L{level}-{seq}.sst
	filename := filepath.Base(path)
	var level, seq int
	fmt.Sscanf(filename, "sst-L%d-%d.sst", &level, &seq)

	return &SSTMetadata{
		Path:     path,
		Level:    level,
		Sequence: seq,
		KeyCount: count,
		FileSize: stat.Size(),
	}, nil
}
