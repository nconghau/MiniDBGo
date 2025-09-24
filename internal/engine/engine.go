package engine

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/your-username/mini-db-go/internal/storage"
)

type Engine struct {
	dbFile *storage.DBFile
	index  map[string]int64 // key -> offset
}

func Open(path string) (*Engine, error) {
	dbf, err := storage.OpenDBFile(path)
	if err != nil {
		return nil, err
	}

	e := &Engine{
		dbFile: dbf,
		index:  make(map[string]int64),
	}

	// rebuild index từ file
	err = dbf.IterateEntries(func(offset int64, key, value []byte) error {
		e.index[string(key)] = offset
		return nil
	})
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (e *Engine) Put(key, value []byte) error {
	offset, err := e.dbFile.WriteEntry(key, value)
	if err != nil {
		return err
	}
	e.index[string(key)] = offset
	return nil
}

func (e *Engine) Get(key []byte) ([]byte, error) {
	offset, ok := e.index[string(key)]
	if !ok {
		return nil, errors.New("key not found")
	}

	// đọc header để biết size
	header := make([]byte, 8)
	if _, err := e.dbFile.File().ReadAt(header, offset); err != nil {
		return nil, err
	}
	kSize := int32(binary.LittleEndian.Uint32(header[:4]))
	vSize := int32(binary.LittleEndian.Uint32(header[4:8]))

	k, v, err := e.dbFile.ReadAt(offset, kSize, vSize)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(k, key) {
		return nil, errors.New("key mismatch (corrupted index?)")
	}
	return v, nil
}

func (e *Engine) Delete(key []byte) error {
	delete(e.index, string(key))
	return nil
}

// ✅ Update (giống Put nhưng semantics rõ ràng)
func (e *Engine) Update(key, value []byte) error {
	offset, err := e.dbFile.WriteEntry(key, value)
	if err != nil {
		return err
	}
	e.index[string(key)] = offset
	return nil
}

func (e *Engine) Index() map[string]int64 {
	return e.index
}
