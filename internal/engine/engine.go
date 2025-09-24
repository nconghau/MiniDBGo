package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"

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

func (e *Engine) Update(key, value []byte) error {
	// update = append bản ghi mới, cập nhật index về offset mới
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
	// optional sanity check
	if !bytes.Equal(k, key) {
		return nil, errors.New("key mismatch (corrupted index?)")
	}
	return v, nil
}

func (e *Engine) Delete(key []byte) error {
	delete(e.index, string(key))
	return nil
}

func (e *Engine) Index() map[string]int64 {
	return e.index
}

/* -------------------------
   findMany + query helpers
   ------------------------- */

// FindMany: trả về list doc (map) trong collection match query
func (e *Engine) FindMany(collection string, query map[string]interface{}) ([]map[string]interface{}, error) {
	results := []map[string]interface{}{}
	for key := range e.index {
		if strings.HasPrefix(key, collection+":") {
			val, err := e.Get([]byte(key))
			if err != nil {
				continue
			}
			var doc map[string]interface{}
			if err := json.Unmarshal(val, &doc); err != nil {
				continue
			}
			if matchDoc(doc, query) {
				results = append(results, doc)
			}
		}
	}
	return results, nil
}

func matchDoc(doc map[string]interface{}, query map[string]interface{}) bool {
	for k, cond := range query {
		fieldVal, ok := doc[k]
		if !ok {
			return false
		}
		if !matchValue(fieldVal, cond) {
			return false
		}
	}
	return true
}

func matchValue(fieldVal interface{}, cond interface{}) bool {
	// cond là operator map?
	if opMap, ok := cond.(map[string]interface{}); ok {
		for op, v := range opMap {
			switch op {
			case "$gt":
				a, ok1 := toFloat(fieldVal)
				b, ok2 := toFloat(v)
				if !ok1 || !ok2 || !(a > b) {
					return false
				}
			case "$gte":
				a, ok1 := toFloat(fieldVal)
				b, ok2 := toFloat(v)
				if !ok1 || !ok2 || !(a >= b) {
					return false
				}
			case "$lt":
				a, ok1 := toFloat(fieldVal)
				b, ok2 := toFloat(v)
				if !ok1 || !ok2 || !(a < b) {
					return false
				}
			case "$lte":
				a, ok1 := toFloat(fieldVal)
				b, ok2 := toFloat(v)
				if !ok1 || !ok2 || !(a <= b) {
					return false
				}
			case "$in":
				arr, ok := v.([]interface{})
				if !ok {
					return false
				}
				found := false
				for _, item := range arr {
					if deepEqualFlex(fieldVal, item) {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			case "$ne":
				if deepEqualFlex(fieldVal, v) {
					return false
				}
			case "$eq":
				if !deepEqualFlex(fieldVal, v) {
					return false
				}
			default:
				// unknown operator => not match
				return false
			}
		}
		return true
	}
	// direct equality
	return deepEqualFlex(fieldVal, cond)
}

func deepEqualFlex(a, b interface{}) bool {
	// try numeric compare first
	if af, oka := toFloat(a); oka {
		if bf, okb := toFloat(b); okb {
			return af == bf
		}
	}
	// fallback to reflect.DeepEqual
	return reflect.DeepEqual(a, b)
}

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	default:
		return 0, false
	}
}

/* -------------------------
   Dump & Restore DB
   ------------------------- */

// DumpDB: export toàn bộ DB vào file (map collection -> [doc, ...])
func (e *Engine) DumpDB(filePath string) error {
	out := map[string][]map[string]interface{}{}
	for key := range e.index {
		val, err := e.Get([]byte(key))
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := json.Unmarshal(val, &doc); err != nil {
			// nếu val không phải json, lưu raw
			doc = map[string]interface{}{"_raw": string(val)}
		}
		// split key => collection:id
		if idx := strings.Index(key, ":"); idx >= 0 {
			collection := key[:idx]
			out[collection] = append(out[collection], doc)
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// RestoreDB: ghi đè DB hiện tại bằng nội dung trong file dump
// Format file: { "collection1": [ {...}, {...} ], "collection2": [...] }
func (e *Engine) RestoreDB(filePath string) error {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	var data map[string][]map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	origPath := e.dbFile.File().Name()
	tmpPath := origPath + ".restore.tmp"
	// make sure tmp not exists
	_ = os.Remove(tmpPath)

	tmpDB, err := storage.OpenDBFile(tmpPath)
	if err != nil {
		return err
	}

	// write all docs
	for collection, docs := range data {
		for _, doc := range docs {
			idVal, ok := doc["_id"]
			if !ok {
				return errors.New("missing _id in document during restore")
			}
			idStr, ok := idVal.(string)
			if !ok {
				return errors.New("_id must be string in restore file")
			}
			docBytes, err := json.Marshal(doc)
			if err != nil {
				return err
			}
			if _, err := tmpDB.WriteEntry([]byte(collection+":"+idStr), docBytes); err != nil {
				return err
			}
		}
	}

	// flush & close files
	if err := tmpDB.File().Sync(); err != nil {
		_ = tmpDB.File().Close()
		return err
	}
	_ = tmpDB.File().Close()
	_ = e.dbFile.File().Close()

	// replace original
	if err := os.Rename(tmpPath, origPath); err != nil {
		return err
	}

	// reopen and rebuild index
	newDB, err := storage.OpenDBFile(origPath)
	if err != nil {
		return err
	}
	e.dbFile = newDB
	e.index = make(map[string]int64)
	err = e.dbFile.IterateEntries(func(offset int64, key, value []byte) error {
		e.index[string(key)] = offset
		return nil
	})
	return err
}

/* -------------------------
   Compaction
   ------------------------- */

func (e *Engine) Compact() error {
	origPath := e.dbFile.File().Name()
	tmpPath := origPath + ".compact.tmp"
	_ = os.Remove(tmpPath)

	tmpDB, err := storage.OpenDBFile(tmpPath)
	if err != nil {
		return err
	}

	// write only latest value per key (index has latest offsets)
	for key := range e.index {
		val, err := e.Get([]byte(key))
		if err != nil {
			continue
		}
		if _, err := tmpDB.WriteEntry([]byte(key), val); err != nil {
			_ = tmpDB.File().Close()
			return err
		}
	}

	// flush & close
	if err := tmpDB.File().Sync(); err != nil {
		_ = tmpDB.File().Close()
		return err
	}
	_ = tmpDB.File().Close()
	_ = e.dbFile.File().Close()

	// replace
	if err := os.Rename(tmpPath, origPath); err != nil {
		return err
	}

	// reopen and rebuild index
	newDB, err := storage.OpenDBFile(origPath)
	if err != nil {
		return err
	}
	e.dbFile = newDB
	e.index = make(map[string]int64)
	err = e.dbFile.IterateEntries(func(offset int64, key, value []byte) error {
		e.index[string(key)] = offset
		return nil
	})
	return err
}
