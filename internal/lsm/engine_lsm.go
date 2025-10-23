package lsm

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type LSMEngine struct {
	dir        string
	wal        *WAL
	mem        *MemTable
	immutMu    sync.RWMutex
	immutables []*MemTable // memtables being flushed
	sstDir     string
	seq        int
	flushSize  int64 // bytes
	mu         sync.RWMutex
}

func OpenLSM(dir string) (*LSMEngine, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	walDir := filepath.Join(dir, "wal")
	sstDir := filepath.Join(dir, "sst")
	_ = os.MkdirAll(walDir, 0o755)
	_ = os.MkdirAll(sstDir, 0o755)

	// identify next WAL seq
	seq := 1
	// naive: check existing files
	files, _ := os.ReadDir(walDir)
	for _, f := range files {
		name := f.Name()
		if strings.HasPrefix(name, "wal-") && strings.HasSuffix(name, ".log") {
			num, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "wal-"), ".log"))
			if num >= seq {
				seq = num + 1
			}
		}
	}
	w, err := OpenWAL(walDir, seq)
	if err != nil {
		return nil, err
	}
	engine := &LSMEngine{
		dir:        dir,
		wal:        w,
		mem:        NewMemTable(),
		immutables: []*MemTable{},
		sstDir:     sstDir,
		seq:        1,
		flushSize:  64 * 1024 * 1024, // 64MB
	}
	// replay existing WALs in walDir (*.log) - simple: replay all files lexicographically
	walFiles, _ := os.ReadDir(walDir)
	names := []string{}
	for _, f := range walFiles {
		if strings.HasPrefix(f.Name(), "wal-") && strings.HasSuffix(f.Name(), ".log") {
			names = append(names, filepath.Join(walDir, f.Name()))
		}
	}
	sort.Strings(names)
	for _, p := range names {
		// open temp wal to iterate (reuse WAL struct would append)
		tmpF, err := os.Open(p)
		if err != nil {
			continue
		}
		wr := &WAL{f: tmpF, path: p}
		_ = wr.Iterate(func(flags byte, key, value []byte) error {
			k := string(key)
			if flags == 1 {
				engine.mem.Delete(k)
			} else {
				engine.mem.Put(k, value)
			}
			return nil
		})
		_ = tmpF.Close()
	}
	return engine, nil
}

// Put writes to WAL and memtable; may trigger flush
func (e *LSMEngine) Put(key, value []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.wal == nil {
		return errors.New("wal not open")
	}
	if err := e.wal.Append(key, value, false); err != nil {
		return err
	}
	e.mem.Put(string(key), value)
	if e.mem.Size() >= e.flushSize {
		return e.rotateAndFlush()
	}
	return nil
}

func (e *LSMEngine) Update(key, value []byte) error {
	return e.Put(key, value)
}

func (e *LSMEngine) Delete(key []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.wal.Append(key, nil, true); err != nil {
		return err
	}
	e.mem.Delete(string(key))
	if e.mem.Size() >= e.flushSize {
		return e.rotateAndFlush()
	}
	return nil
}

// Get checks memtable, immutables (recent snapshots), then SST files (newest first)
func (e *LSMEngine) Get(key []byte) ([]byte, error) {
	k := string(key)
	// check memtable
	if it, ok := e.mem.Get(k); ok {
		if it.Tombstone {
			return nil, errors.New("key not found")
		}
		return it.Value, nil
	}
	// check immutables
	e.immutMu.RLock()
	for _, m := range e.immutables {
		if it, ok := m.Get(k); ok {
			if it.Tombstone {
				e.immutMu.RUnlock()
				return nil, errors.New("key not found")
			}
			e.immutMu.RUnlock()
			return it.Value, nil
		}
	}
	e.immutMu.RUnlock()

	// scan SST files newest-first
	files, err := os.ReadDir(e.sstDir)
	if err == nil {
		// sort descending by name so newest are checked first
		names := make([]string, 0, len(files))
		for _, fi := range files {
			if strings.HasSuffix(fi.Name(), ".sst") {
				names = append(names, filepath.Join(e.sstDir, fi.Name()))
			}
		}
		sort.Slice(names, func(i, j int) bool { return names[i] > names[j] })
		for _, p := range names {
			if bv, tomb, err := ReadSSTFind(p, k); err == nil {
				if tomb {
					return nil, errors.New("key not found")
				}
				if bv != nil {
					return bv, nil
				}
			}
		}
	}
	return nil, errors.New("key not found")
}

// IterKeys returns all keys from memtable + sst (careful with large DB)
func (e *LSMEngine) IterKeys() ([]string, error) {
	keysMap := map[string]struct{}{}
	// memtable
	for _, k := range e.mem.Keys() {
		keysMap[k] = struct{}{}
	}
	// immutables
	e.immutMu.RLock()
	for _, m := range e.immutables {
		for _, k := range m.Keys() {
			keysMap[k] = struct{}{}
		}
	}
	e.immutMu.RUnlock()
	// sst (naive scan)
	files, _ := os.ReadDir(e.sstDir)
	for _, fi := range files {
		if !strings.HasSuffix(fi.Name(), ".sst") {
			continue
		}
		// naive: open and scan file sequentially (not efficient but works)
		p := filepath.Join(e.sstDir, fi.Name())
		// reuse ReadSSTFind by scanning? Simpler: open and parse quickly
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		// skip header
		h := make([]byte, 8)
		if _, err := f.Read(h); err != nil {
			f.Close()
			continue
		}
		var count uint32
		if err := binary.Read(f, binary.LittleEndian, &count); err != nil {
			f.Close()
			continue
		}
		for i := uint32(0); i < count; i++ {
			var klen uint32
			var vlen uint32
			if err := binary.Read(f, binary.LittleEndian, &klen); err != nil {
				break
			}
			if err := binary.Read(f, binary.LittleEndian, &vlen); err != nil {
				break
			}
			flag := make([]byte, 1)
			if _, err := f.Read(flag); err != nil {
				break
			}
			kb := make([]byte, klen)
			if _, err := f.Read(kb); err != nil {
				break
			}
			if _, err := f.Seek(int64(vlen), io.SeekCurrent); err != nil {
				break
			}
			keysMap[string(kb)] = struct{}{}
		}
		_ = f.Close()
	}
	keys := make([]string, 0, len(keysMap))
	for k := range keysMap {
		keys = append(keys, k)
	}
	return keys, nil
}

// rotateAndFlush: snapshot current memtable to immutable, spawn flush goroutine
func (e *LSMEngine) rotateAndFlush() error {
	// create snapshot
	snap := e.mem
	e.mem = NewMemTable()
	// mark immutable
	e.immutMu.Lock()
	e.immutables = append(e.immutables, snap)
	e.immutMu.Unlock()

	// flush synchronously for simplicity (could be background)
	items := snap.SnapshotAndReset()
	if len(items) == 0 {
		// remove from immutables
		e.immutMu.Lock()
		if len(e.immutables) > 0 {
			e.immutables = e.immutables[:len(e.immutables)-1]
		}
		e.immutMu.Unlock()
		return nil
	}
	// write SST
	seq := e.seq
	e.seq++
	_, err := WriteSST(e.sstDir, 0, seq, items)
	// remove immutable reference
	e.immutMu.Lock()
	// remove first match
	for i, m := range e.immutables {
		if m == snap {
			e.immutables = append(e.immutables[:i], e.immutables[i+1:]...)
			break
		}
	}
	e.immutMu.Unlock()
	return err
}

// DumpDB simple: collect all keys via IterKeys and read their values
func (e *LSMEngine) DumpDB(path string) error {
	keys, err := e.IterKeys()
	if err != nil {
		return err
	}
	out := map[string]map[string][]byte{} // collection -> id -> raw doc bytes
	for _, full := range keys {
		// full = collection:id
		if idx := strings.Index(full, ":"); idx >= 0 {
			collection := full[:idx]
			id := full[idx+1:]
			v, err := e.Get([]byte(full))
			if err != nil {
				continue
			}
			if _, ok := out[collection]; !ok {
				out[collection] = map[string][]byte{}
			}
			out[collection][id] = append([]byte(nil), v...)
		}
	}
	// convert to map[string][]json doc
	final := map[string][]map[string]interface{}{}
	for col, entries := range out {
		for id, raw := range entries {
			var doc map[string]interface{}
			_ = json.Unmarshal(raw, &doc)
			// ensure _id present
			if doc == nil {
				doc = map[string]interface{}{"_id": id, "_raw": string(raw)}
			} else {
				doc["_id"] = id
			}
			final[col] = append(final[col], doc)
		}
	}
	data, _ := json.MarshalIndent(final, "", "  ")
	return os.WriteFile(path, data, 0o644)
}

// RestoreDB: naive implementation - writes to new memtable via Put
func (e *LSMEngine) RestoreDB(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var data map[string][]map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	for col, docs := range data {
		for _, doc := range docs {
			idV, ok := doc["_id"]
			if !ok {
				return fmt.Errorf("missing _id in doc for collection %s", col)
			}
			idStr, ok := idV.(string)
			if !ok {
				return fmt.Errorf("_id must be string in restore file")
			}
			raw, _ := json.Marshal(doc)
			if err := e.Put([]byte(col+":"+idStr), raw); err != nil {
				return err
			}
		}
	}
	return nil
}
