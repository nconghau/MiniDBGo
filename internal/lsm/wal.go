package lsm

import (
	"bufio"
	"encoding/binary"
	"fmt"
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

	// Flush after every write for durability
	return w.w.Flush()
}

// Iterate to replay WAL
func (w *WAL) Iterate(fn func(flag byte, key, value []byte) error) error {
	if _, err := w.f.Seek(0, 0); err != nil {
		return err
	}
	r := bufio.NewReaderSize(w.f, 256*1024)

	for {
		var klen, vlen uint32
		if err := binary.Read(r, binary.LittleEndian, &klen); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &vlen); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		flag, err := r.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		key := make([]byte, klen)
		if _, err := io.ReadFull(r, key); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		val := make([]byte, vlen)
		if _, err := io.ReadFull(r, val); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

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
