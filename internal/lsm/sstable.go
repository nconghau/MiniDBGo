package lsm

import (
	"bufio"
	"bytes" // --- MỚI ---
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/nconghau/MiniDBGo/internal/engine"
)

const (
	// SSTable format version
	SSTVersion = 1

	// Buffer sizes
	SSTWriteBufferSize = 256 * 1024 // 256KB
	SSTReadBufferSize  = 128 * 1024 // 128KB

	// --- MỚI: Kích thước khối dữ liệu ---
	SSTDataBlockSize = 4 * 1024 // 4KB

	// SSTable file format:
	// [Header: 8 bytes]
	// [Data Block 1]
	// [Data Block 2]
	// ...
	// [Index Block: variable]
	// [BloomFilter Data: variable]
	// [Footer: 44 bytes]
	//
	// Header: version(4) + count(4)
	// Entry: keyLen(4) + valueLen(4) + flag(1) + key + value
	//
	// --- SỬA ĐỔI: Footer ---
	// Footer: indexOffset(8) + indexLen(8) + bloomOffset(8) + bloomLen(8) + bloomN_bits(8) + bloomK_hashes(4)
	SSTFooterSize = 44 // 8+8+8+8+8+4
)

// --- MỚI: Cấu trúc cho một entry trong Index Block ---
type blockIndexEntry struct {
	lastKey string // Khóa cuối cùng trong khối dữ liệu
	offset  int64  // Offset bắt đầu của khối dữ liệu
	length  int64  // Độ dài của khối dữ liệu
}

// SSTMetadata (Không thay đổi)
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

	// --- MỚI: Trạng thái cho Block Index ---
	indexEntries       []blockIndexEntry // Danh sách các entry index
	currentBlock       bytes.Buffer      // Bộ đệm cho khối dữ liệu hiện tại
	currentBlockOffset int64             // Offset tệp nơi khối hiện tại bắt đầu
	lastBlockKey       string            // Khóa cuối cùng được ghi vào khối hiện tại
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
		bloom:  NewBloomFilter(estimatedKeys*10, 3), // [cite: 87]

		// --- MỚI: Khởi tạo trạng thái Block Index ---
		indexEntries:       make([]blockIndexEntry, 0, 128),
		currentBlock:       bytes.Buffer{},
		currentBlockOffset: 8, // Bắt đầu sau header 8 byte
	}

	// Write header placeholder (will be updated on close)
	header := make([]byte, 8)
	binary.LittleEndian.PutUint32(header[0:4], SSTVersion)
	binary.LittleEndian.PutUint32(header[4:8], 0)     // count placeholder
	if _, err := w.writer.Write(header); err != nil { // [cite: 88]
		f.Close()
		return nil, fmt.Errorf("write header: %w", err)
	}

	return w, nil
}

// --- MỚI: Hàm flush khối dữ liệu hiện tại ra đĩa ---
func (w *SSTWriter) flushCurrentBlock() error {
	if w.currentBlock.Len() == 0 {
		return nil
	}

	blockData := w.currentBlock.Bytes()

	// --- LOGIC MỚI: TÍNH VÀ GHI CRC ---
	crc := crc32.Checksum(blockData, crcTable)
	// --- KẾT THÚC LOGIC MỚI ---

	// Ghi khối dữ liệu (như cũ)
	if _, err := w.writer.Write(blockData); err != nil {
		return fmt.Errorf("write data block: %w", err)
	}

	// --- LOGIC MỚI: GHI CRC (4 bytes) ---
	if err := binary.Write(w.writer, binary.LittleEndian, crc); err != nil {
		return fmt.Errorf("write data block crc: %w", err)
	}
	// --- KẾT THÚC LOGIC MỚI ---

	w.indexEntries = append(w.indexEntries, blockIndexEntry{
		lastKey: w.lastBlockKey,
		offset:  w.currentBlockOffset,
		length:  int64(len(blockData)), // Vẫn giữ nguyên length của data
	})

	// Cập nhật offset cho khối tiếp theo
	// (offset MỚI = offset cũ + data_len + 4 byte CRC)
	w.currentBlockOffset += int64(len(blockData)) + 4 // +4 cho CRC
	w.currentBlock.Reset()
	return nil
}

// WriteEntry writes a single key-value entry
// --- SỬA ĐỔI: Ghi vào bộ đệm khối (block buffer) ---
func (w *SSTWriter) WriteEntry(key string, item *engine.Item) error {
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

	// --- SỬA ĐỔI: Ghi entry vào bộ đệm khối (currentBlock) ---
	// Định dạng entry không đổi: keyLen(4) + valueLen(4) + flag(1) + key + value
	entryHeader := make([]byte, 9) // 4+4+1
	binary.LittleEndian.PutUint32(entryHeader[0:4], uint32(len(kb)))
	binary.LittleEndian.PutUint32(entryHeader[4:8], uint32(len(vb)))
	if item.Tombstone {
		entryHeader[8] = 1
	} else {
		entryHeader[8] = 0
	}

	w.currentBlock.Write(entryHeader)
	w.currentBlock.Write(kb)
	w.currentBlock.Write(vb)

	w.lastBlockKey = key
	// --- KẾT THÚC SỬA ĐỔI ---

	// Nếu khối đầy, flush nó
	if w.currentBlock.Len() >= SSTDataBlockSize {
		if err := w.flushCurrentBlock(); err != nil {
			return err
		}
	}

	return nil
}

// Close finalizes the SSTable file
// --- SỬA ĐỔI: Ghi Index Block, Bloom Filter, và Footer mới ---
func (w *SSTWriter) Close() error {
	// 1. Flush khối dữ liệu cuối cùng (nếu còn)
	if err := w.flushCurrentBlock(); err != nil {
		return fmt.Errorf("flush final block: %w", err)
	}

	// 2. Flush mọi thứ từ writer ra file
	if err := w.writer.Flush(); err != nil { // [cite: 93]
		return fmt.Errorf("flush writer: %w", err)
	}

	// 3. Ghi Index Block
	indexOffset, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("seek for index block: %w", err)
	}

	// Ghi số lượng entry trong index
	if err := binary.Write(w.file, binary.LittleEndian, uint32(len(w.indexEntries))); err != nil {
		return fmt.Errorf("write index entry count: %w", err)
	}
	// Ghi từng entry (lastKeyLen, lastKey, offset, length)
	for _, entry := range w.indexEntries {
		keyBytes := []byte(entry.lastKey)
		// keyLen
		if err := binary.Write(w.file, binary.LittleEndian, uint32(len(keyBytes))); err != nil {
			return fmt.Errorf("write index keylen: %w", err)
		}
		// key
		if _, err := w.file.Write(keyBytes); err != nil {
			return fmt.Errorf("write index key: %w", err)
		}
		// offset
		if err := binary.Write(w.file, binary.LittleEndian, entry.offset); err != nil {
			return fmt.Errorf("write index offset: %w", err)
		}
		// length
		if err := binary.Write(w.file, binary.LittleEndian, entry.length); err != nil {
			return fmt.Errorf("write index length: %w", err)
		}
	}

	indexEndOffset, _ := w.file.Seek(0, io.SeekCurrent)
	indexLen := indexEndOffset - indexOffset

	// 4. Ghi Bloom Filter (như Giai đoạn 1)
	bloomOffset, err := w.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("seek for bloom offset: %w", err)
	}
	bloomData := w.bloom.ToBytes()
	if _, err := w.file.Write(bloomData); err != nil {
		return fmt.Errorf("write bloom data: %w", err)
	}
	bloomLen := uint64(len(bloomData))

	// 5. Ghi Footer mới (44 bytes)
	// indexOffset(8) + indexLen(8) + bloomOffset(8) + bloomLen(8) + bloomN_bits(8) + bloomK_hashes(4)
	if err := binary.Write(w.file, binary.LittleEndian, uint64(indexOffset)); err != nil {
		return fmt.Errorf("write footer index offset: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, uint64(indexLen)); err != nil {
		return fmt.Errorf("write footer index length: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, uint64(bloomOffset)); err != nil {
		return fmt.Errorf("write footer bloom offset: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, bloomLen); err != nil {
		return fmt.Errorf("write footer bloom length: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, uint64(w.bloom.n)); err != nil {
		return fmt.Errorf("write footer bloom N: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, uint32(w.bloom.k)); err != nil {
		return fmt.Errorf("write footer bloom K: %w", err)
	}

	// 6. Cập nhật Header (như cũ)
	if _, err := w.file.Seek(4, io.SeekStart); err != nil { // [cite: 94]
		return fmt.Errorf("seek header: %w", err)
	}
	if err := binary.Write(w.file, binary.LittleEndian, w.count); err != nil { // [cite: 95]
		return fmt.Errorf("write count: %w", err)
	}

	// 7. Sync và Close (như cũ)
	if err := w.file.Sync(); err != nil { // [cite: 96]
		return fmt.Errorf("sync file: %w", err)
	}

	return w.file.Close()
}

// --- KẾT THÚC SỬA ĐỔI Close() ---

// GetMetadata (Không thay đổi)
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

// WriteSST (Không thay đổi)
func WriteSST(dir string, level, seq int, items map[string]*engine.Item) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("no items to write")
	}
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	path := filepath.Join(dir, fmt.Sprintf("sst-L%d-%06d.sst", level, seq)) // [cite: 97]
	writer, err := NewSSTWriter(path, uint32(len(items)))
	if err != nil {
		return "", err
	}
	for _, key := range keys {
		if err := writer.WriteEntry(key, items[key]); err != nil { // [cite: 98]
			writer.Close()
			os.Remove(path)
			return "", fmt.Errorf("write entry %s: %w", key, err)
		}
	}
	if err := writer.Close(); err != nil { // [cite: 99]
		os.Remove(path)
		return "", fmt.Errorf("close writer: %w", err)
	}
	return path, nil
}

// --- MỚI: Hàm đọc và tìm kiếm trong một khối dữ liệu ---
func searchDataBlock(blockData []byte, key string) ([]byte, bool, error) {
	r := bytes.NewReader(blockData)
	keyBytes := []byte(key)

	for r.Len() > 0 {
		var klen, vlen uint32
		var flag byte
		var err error // --- SỬA 1: Khai báo 'err' một lần ở đây ---

		if err = binary.Read(r, binary.LittleEndian, &klen); err != nil { // --- SỬA 2: Sử dụng '=' ---
			return nil, false, fmt.Errorf("read data keylen: %w", err)
		}
		if err = binary.Read(r, binary.LittleEndian, &vlen); err != nil { // --- SỬA 3: Sử dụng '=' ---
			return nil, false, fmt.Errorf("read data vallen: %w", err)
		}

		// --- SỬA 4: Sử dụng gán '=' để gán giá trị cho 'flag' và 'err' đã khai báo bên ngoài ---
		flag, err = r.ReadByte()
		if err != nil {
			return nil, false, fmt.Errorf("read data flag: %w", err)
		}

		kb := make([]byte, klen)
		if _, err := io.ReadFull(r, kb); err != nil {
			return nil, false, fmt.Errorf("read data key: %w", err)
		}

		if bytes.Equal(kb, keyBytes) {
			vb := make([]byte, vlen)
			if vlen > 0 {
				if _, err := io.ReadFull(r, vb); err != nil {
					return nil, false, fmt.Errorf("read data value: %w", err)
				}
			}
			if flag == 1 { // 'flag' bây giờ là biến được gán giá trị chính xác
				return nil, true, nil // tombstone
			}
			return vb, false, nil
		} else {
			// Bỏ qua value nếu key không khớp
			if _, err := r.Seek(int64(vlen), io.SeekCurrent); err != nil {
				return nil, false, fmt.Errorf("skip data value: %w", err)
			}
		}
	}

	return nil, false, os.ErrNotExist
}

// --- MỚI: Hàm đọc Index Block và tìm khối dữ liệu phù hợp ---
func readAndSearchIndexBlock(f *os.File, indexOffset, indexLen int64, key string) (int64, int64, error) {
	indexData := make([]byte, indexLen)
	if _, err := f.ReadAt(indexData, indexOffset); err != nil {
		return 0, 0, fmt.Errorf("read index block: %w", err)
	}

	r := bytes.NewReader(indexData)
	var numEntries uint32
	if err := binary.Read(r, binary.LittleEndian, &numEntries); err != nil {
		return 0, 0, fmt.Errorf("read index entry count: %w", err)
	}

	// Đọc tất cả các entry vào bộ nhớ (vì index block thường nhỏ)
	entries := make([]blockIndexEntry, numEntries)
	for i := 0; i < int(numEntries); i++ {
		var klen uint32
		if err := binary.Read(r, binary.LittleEndian, &klen); err != nil {
			return 0, 0, fmt.Errorf("read index entry klen: %w", err)
		}
		keyBytes := make([]byte, klen)
		if _, err := io.ReadFull(r, keyBytes); err != nil {
			return 0, 0, fmt.Errorf("read index entry key: %w", err)
		}
		entries[i].lastKey = string(keyBytes)
		if err := binary.Read(r, binary.LittleEndian, &entries[i].offset); err != nil {
			return 0, 0, fmt.Errorf("read index entry offset: %w", err)
		}
		if err := binary.Read(r, binary.LittleEndian, &entries[i].length); err != nil {
			return 0, 0, fmt.Errorf("read index entry length: %w", err)
		}
	}

	// Tìm kiếm nhị phân (Binary Search)
	// Tìm khối *đầu tiên* mà lastKey >= key
	i := sort.Search(len(entries), func(i int) bool {
		return entries[i].lastKey >= key
	})

	if i == len(entries) {
		// Key lớn hơn tất cả các lastKey, không có trong tệp này
		return 0, 0, os.ErrNotExist
	}

	// Trả về offset và length của khối dữ liệu đã tìm thấy
	return entries[i].offset, entries[i].length, nil
}

// ReadSSTFind searches for a key in an SSTable file
// --- SỬA ĐỔI: Sử dụng Index Block thay vì quét tuần tự ---
func ReadSSTFind(path string, key string) ([]byte, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, false, err
	}

	// 1. Đọc Footer
	if stat.Size() < (8 + SSTFooterSize) {
		// Tệp quá nhỏ, có thể đang trong quá trình ghi hoặc bị hỏng
		return nil, false, fmt.Errorf("file too small or corrupt")
	}

	footerData := make([]byte, SSTFooterSize)
	if _, err := f.ReadAt(footerData, stat.Size()-SSTFooterSize); err != nil {
		return nil, false, fmt.Errorf("read footer: %w", err)
	}

	var indexOffset, indexLen, bloomOffset, bloomLen, bloomN uint64
	var bloomK uint32

	r := bytes.NewReader(footerData)
	binary.Read(r, binary.LittleEndian, &indexOffset)
	binary.Read(r, binary.LittleEndian, &indexLen)
	binary.Read(r, binary.LittleEndian, &bloomOffset)
	binary.Read(r, binary.LittleEndian, &bloomLen)
	binary.Read(r, binary.LittleEndian, &bloomN)
	binary.Read(r, binary.LittleEndian, &bloomK)

	// 2. Kiểm tra Bloom Filter
	bloomData := make([]byte, bloomLen)
	if _, err = f.ReadAt(bloomData, int64(bloomOffset)); err != nil {
		return nil, false, fmt.Errorf("read bloom data: %w", err)
	}

	bloom := NewFromBytes(bloomData, uint32(bloomN), int(bloomK))
	if !bloom.MightContain(key) {
		return nil, false, os.ErrNotExist // Tối ưu hóa thành công!
	}

	// 3. Đọc Index Block và tìm Data Block
	blockOffset, blockLen, err := readAndSearchIndexBlock(f, int64(indexOffset), int64(indexLen), key)
	if err != nil {
		return nil, false, err // os.ErrNotExist nếu không tìm thấy
	}

	// 4. Đọc và quét Data Block
	dataBlock := make([]byte, blockLen)
	if _, err := f.ReadAt(dataBlock, blockOffset); err != nil {
		return nil, false, fmt.Errorf("read data block: %w", err)
	}

	// --- LOGIC MỚI: ĐỌC VÀ KIỂM TRA CRC ---
	var storedCrc uint32
	// Đọc 4 byte CRC ngay sau data block
	crcBytes := make([]byte, 4)
	if _, err := f.ReadAt(crcBytes, blockOffset+blockLen); err != nil {
		return nil, false, fmt.Errorf("read data block crc: %w", err)
	}

	if err := binary.Read(bytes.NewReader(crcBytes), binary.LittleEndian, &storedCrc); err != nil {
		return nil, false, fmt.Errorf("parse data block crc: %w", err)
	}

	calculatedCrc := crc32.Checksum(dataBlock, crcTable)
	if storedCrc != calculatedCrc {
		return nil, false, ErrCorruption // Lỗi! Block SSTable bị hỏng.
	}
	// --- KẾT THÚC LOGIC MỚI ---

	// Sử dụng hàm đã sửa lỗi
	return searchDataBlock(dataBlock, key)

	// --- TOÀN BỘ LOGIC QUÉT TUẦN TỰ GỐC ĐÃ BỊ XÓA ---
}
