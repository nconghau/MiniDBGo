package lsm

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Compact merges all SST files at level 0 into one bigger SST
func (e *LSMEngine) Compact() error {
	files, err := os.ReadDir(e.sstDir)
	if err != nil {
		return err
	}
	var ssts []string
	for _, fi := range files {
		if strings.HasSuffix(fi.Name(), ".sst") {
			ssts = append(ssts, filepath.Join(e.sstDir, fi.Name()))
		}
	}
	if len(ssts) <= 1 {
		return nil // nothing to compact
	}
	sort.Strings(ssts)

	merged := map[string]*Item{}
	for _, path := range ssts {
		fitems, err := readAllSST(path)
		if err != nil {
			continue
		}
		for k, v := range fitems {
			merged[k] = v // overwrite older
		}
	}

	// remove tombstones
	clean := map[string]*Item{}
	for k, v := range merged {
		if !v.Tombstone {
			clean[k] = v
		}
	}

	// write new SST
	e.mu.Lock() // Protect access to e.seq
	e.seq++
	seq := e.seq
	e.mu.Unlock()

	newPath, err := WriteSST(e.sstDir, 1, seq, clean)
	if err != nil {
		return err
	}

	e.metrics.compacts.Add(1)

	// delete old files
	for _, p := range ssts {
		if p != newPath {
			_ = os.Remove(p)
		}
	}
	return nil
}

// helper: load entire SST
func readAllSST(path string) (map[string]*Item, error) {
	res := map[string]*Item{}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var count uint32
	if err := binary.Read(f, binary.LittleEndian, &count); err != nil {
		return nil, err
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

		res[string(kb)] = &Item{
			Value:     vb,
			Tombstone: flag[0] == 1,
		}
	}
	return res, nil
}
