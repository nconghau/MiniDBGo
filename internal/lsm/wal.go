package lsm

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type WAL struct {
	f    *os.File
	path string
	w    *bufio.Writer
	mu   sync.Mutex
}

func OpenWAL(dir string, seq int) (*WAL, error) {
	path := filepath.Join(dir, fmt.Sprintf("wal-%d.log", seq))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &WAL{
		f:    f,
		path: path,
		w:    bufio.NewWriterSize(f, 256*1024), // 256KB buffer
	}, nil
}

// Append an entry (delete=true means tombstone)
func (w *WAL) Append(key, value []byte, delete bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	flag := byte(0)
	if delete {
		flag = 1
	}

	// --- LOGIC MỚI BẮT ĐẦU ---
	// 1. Tạo buffer cho dữ liệu cần checksum
	// (flag + key + value)
	dataLen := 1 + len(key) + len(value)
	buf := make([]byte, dataLen)
	buf[0] = flag
	copy(buf[1:], key)
	copy(buf[1+len(key):], value)

	// 2. Tính CRC
	crc := crc32.Checksum(buf, crcTable)
	// --- LOGIC MỚI KẾT THÚC ---

	// 3. Ghi CRC (MỚI)
	if err := binary.Write(w.w, binary.LittleEndian, crc); err != nil {
		return err
	}
	// 4. Ghi phần còn lại (như cũ)
	if err := binary.Write(w.w, binary.LittleEndian, uint32(len(key))); err != nil {
		return err
	}
	if err := binary.Write(w.w, binary.LittleEndian, uint32(len(value))); err != nil {
		return err
	}
	if _, err := w.w.Write([]byte{flag}); err != nil {
		return err
	}
	if _, err := w.w.Write(key); err != nil {
		return err
	}
	if _, err := w.w.Write(value); err != nil {
		return err
	}

	return w.w.Flush()
}

// Iterate to replay WAL
func (w *WAL) Iterate(fn func(flag byte, key, value []byte) error) error {
	if _, err := w.f.Seek(0, 0); err != nil {
		return err
	}
	r := bufio.NewReaderSize(w.f, 256*1024)

	// Buffer tái sử dụng để tính toán CRC
	buf := make([]byte, 1024)

	for {
		// --- LOGIC MỚI: ĐỌC VÀ KIỂM TRA CRC ---
		var storedCrc uint32
		if err := binary.Read(r, binary.LittleEndian, &storedCrc); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		// --- KẾT THÚC LOGIC MỚI ---

		var klen, vlen uint32
		if err := binary.Read(r, binary.LittleEndian, &klen); err != nil {
			return err // Báo lỗi (hỏng hóc) nếu file kết thúc đột ngột sau CRC
		}
		if err := binary.Read(r, binary.LittleEndian, &vlen); err != nil {
			return err
		}

		flag, err := r.ReadByte()
		if err != nil {
			return err
		}

		key := make([]byte, klen)
		if _, err := io.ReadFull(r, key); err != nil {
			return err
		}

		val := make([]byte, vlen)
		if _, err := io.ReadFull(r, val); err != nil {
			return err
		}

		// --- LOGIC MỚI: XÁC THỰC CRC ---
		dataLen := 1 + klen + vlen
		if cap(buf) < int(dataLen) {
			buf = make([]byte, dataLen)
		}
		buf = buf[:dataLen] // Chỉnh kích thước

		buf[0] = flag
		copy(buf[1:], key)
		copy(buf[1+klen:], val)

		calculatedCrc := crc32.Checksum(buf, crcTable)

		if storedCrc != calculatedCrc {
			return ErrCorruption // Lỗi! Dữ liệu WAL đã bị hỏng.
		}
		// --- KẾT THÚC LOGIC MỚI ---

		if err := fn(flag, key, val); err != nil {
			return err
		}
	}
	return nil
}

// Close flushes and closes the WAL file
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.w != nil {
		if err := w.w.Flush(); err != nil {
			return err
		}
	}

	if w.f != nil {
		if err := w.f.Sync(); err != nil {
			return err
		}
		return w.f.Close()
	}

	return nil
}
