package lsm

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

func WriteSST(dir string, level, seq int, items map[string]*Item) (string, error) {
	path := filepath.Join(dir,
		"sst-"+
			fmt.Sprintf("%d-%d.sst", level, seq))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	count := uint32(len(items))
	if err := binary.Write(f, binary.LittleEndian, count); err != nil {
		return "", err
	}

	for k, v := range items {
		kb := []byte(k)
		vb := v.Value
		if v.Tombstone {
			vb = []byte{}
		}
		binary.Write(f, binary.LittleEndian, uint32(len(kb)))
		binary.Write(f, binary.LittleEndian, uint32(len(vb)))
		flag := byte(0)
		if v.Tombstone {
			flag = 1
		}
		f.Write([]byte{flag})
		f.Write(kb)
		f.Write(vb)
	}
	return path, nil
}

// ReadSSTFind scans SST for a key
func ReadSSTFind(path string, key string) ([]byte, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	var count uint32
	if err := binary.Read(f, binary.LittleEndian, &count); err != nil {
		return nil, false, err
	}
	for i := uint32(0); i < count; i++ {
		var klen, vlen uint32
		binary.Read(f, binary.LittleEndian, &klen)
		binary.Read(f, binary.LittleEndian, &vlen)
		flag := make([]byte, 1)
		f.Read(flag)
		kb := make([]byte, klen)
		f.Read(kb)
		vb := make([]byte, vlen)
		f.Read(vb)
		if string(kb) == key {
			if flag[0] == 1 {
				return nil, true, nil // tombstone
			}
			return vb, false, nil
		}
	}
	return nil, false, os.ErrNotExist
}
