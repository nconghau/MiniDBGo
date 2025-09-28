package lsm

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type WAL struct {
	f    *os.File
	path string
	w    *bufio.Writer
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
		w:    bufio.NewWriter(f),
	}, nil
}

// Append an entry (delete=true means tombstone)
func (w *WAL) Append(key, value []byte, delete bool) error {
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
	return w.w.Flush()
}

// Iterate to replay WAL
func (w *WAL) Iterate(fn func(flag byte, key, value []byte) error) error {
	if _, err := w.f.Seek(0, 0); err != nil {
		return err
	}
	r := bufio.NewReader(w.f)
	for {
		var klen, vlen uint32
		if err := binary.Read(r, binary.LittleEndian, &klen); err != nil {
			break
		}
		if err := binary.Read(r, binary.LittleEndian, &vlen); err != nil {
			break
		}
		flag, _ := r.ReadByte()
		key := make([]byte, klen)
		if _, err := io.ReadFull(r, key); err != nil {
			break
		}
		val := make([]byte, vlen)
		if _, err := io.ReadFull(r, val); err != nil {
			break
		}
		if err := fn(flag, key, val); err != nil {
			return err
		}
	}
	return nil
}
