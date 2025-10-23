package storage

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
)

type DBFile struct {
	file *os.File
}

func OpenDBFile(path string) (*DBFile, error) {
	// Đảm bảo thư mục cha tồn tại
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &DBFile{file: f}, nil
}

func (dbf *DBFile) File() *os.File {
	return dbf.file
}

func (dbf *DBFile) WriteEntry(key, value []byte) (int64, error) {
	pos, _ := dbf.file.Seek(0, os.SEEK_END)

	kSize := int32(len(key))
	vSize := int32(len(value))

	if err := binary.Write(dbf.file, binary.LittleEndian, kSize); err != nil {
		return 0, err
	}
	if err := binary.Write(dbf.file, binary.LittleEndian, vSize); err != nil {
		return 0, err
	}
	if _, err := dbf.file.Write(key); err != nil {
		return 0, err
	}
	if _, err := dbf.file.Write(value); err != nil {
		return 0, err
	}

	return pos, nil
}

func (dbf *DBFile) ReadEntryAt(offset int64) ([]byte, []byte, error) {
	var kSize, vSize int32

	// Đọc độ dài key và value (8 bytes đầu)
	if _, err := dbf.file.Seek(offset, io.SeekStart); err != nil {
		return nil, nil, err
	}
	if err := binary.Read(dbf.file, binary.LittleEndian, &kSize); err != nil {
		return nil, nil, err
	}
	if err := binary.Read(dbf.file, binary.LittleEndian, &vSize); err != nil {
		return nil, nil, err
	}

	// Tạo buffer và đọc key + value
	buf := make([]byte, kSize+vSize)
	if _, err := dbf.file.Read(buf); err != nil {
		return nil, nil, err
	}

	// Cắt ra
	return buf[:kSize], buf[kSize:], nil
}

// IterateEntries scan toàn bộ file để rebuild index
func (dbf *DBFile) IterateEntries(fn func(offset int64, key, value []byte) error) error {
	var offset int64 = 0
	header := make([]byte, 8)

	for {
		_, err := dbf.file.ReadAt(header, offset)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		kSize := int32(binary.LittleEndian.Uint32(header[:4]))
		vSize := int32(binary.LittleEndian.Uint32(header[4:8]))

		data := make([]byte, kSize+vSize)
		if _, err := dbf.file.ReadAt(data, offset+8); err != nil {
			return err
		}

		key := data[:kSize]
		value := data[kSize:]

		if err := fn(offset, key, value); err != nil {
			return err
		}

		offset += int64(8 + kSize + vSize)
	}

	return nil
}
