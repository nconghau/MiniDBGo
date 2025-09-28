package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/your-username/mini-db-go/internal/lsm"
)

// helper: parse "<collection> <one JSON>" where JSON may contain spaces
func parseCollectionAndOneJSON(rest string) (collection string, doc map[string]interface{}, err error) {
	rest = strings.TrimSpace(rest)
	// collection is first token
	idx := strings.IndexAny(rest, " \t")
	if idx == -1 {
		return "", nil, fmt.Errorf("missing collection or JSON")
	}
	collection = rest[:idx]
	jsonPart := strings.TrimSpace(rest[idx+1:])
	dec := json.NewDecoder(strings.NewReader(jsonPart))
	dec.UseNumber()
	if err = dec.Decode(&doc); err != nil {
		return "", nil, err
	}
	return collection, doc, nil
}

// helper: parse "<collection> <json1> <json2>" where json1/json2 may contain spaces
func parseCollectionAndTwoJSON(rest string) (collection string, j1 map[string]interface{}, j2 map[string]interface{}, err error) {
	rest = strings.TrimSpace(rest)
	idx := strings.IndexAny(rest, " \t")
	if idx == -1 {
		return "", nil, nil, fmt.Errorf("missing collection or JSONs")
	}
	collection = rest[:idx]
	remain := strings.TrimSpace(rest[idx+1:])
	dec := json.NewDecoder(strings.NewReader(remain))
	dec.UseNumber()
	if err = dec.Decode(&j1); err != nil {
		return collection, nil, nil, fmt.Errorf("invalid first JSON: %w", err)
	}
	if err = dec.Decode(&j2); err != nil {
		return collection, j1, nil, fmt.Errorf("invalid second JSON: %w", err)
	}
	return collection, j1, j2, nil
}

// insertOne <collection> <json>
func handleInsertOne(db *lsm.LSMEngine, rest string) {
	collection, doc, err := parseCollectionAndOneJSON(rest)
	if err != nil {
		fmt.Println("Usage: insertOne <collection> <json>   — parse error:", err)
		return
	}
	idVal, ok := doc["_id"]
	if !ok {
		fmt.Println("Document must contain _id field")
		return
	}
	id := fmt.Sprintf("%v", idVal)
	raw, _ := json.Marshal(doc)
	if err := db.Put([]byte(collection+":"+id), raw); err != nil {
		fmt.Println("Insert error:", err)
		return
	}
	fmt.Printf("Inserted into %s with _id=%s\n", collection, id)
}

// findOne <collection> <jsonFilter>
func handleFindOne(db *lsm.LSMEngine, rest string) {
	collection, filter, err := parseCollectionAndOneJSON(rest)
	if err != nil {
		fmt.Println("Usage: findOne <collection> <jsonFilter> — parse error:", err)
		return
	}
	idVal, ok := filter["_id"]
	if !ok {
		fmt.Println("findOne currently supports {_id:...}")
		return
	}
	id := fmt.Sprintf("%v", idVal)
	val, err := db.Get([]byte(collection + ":" + id))
	if err != nil {
		fmt.Println("Not found")
		return
	}
	fmt.Println(string(val))
}

// findMany <collection> <jsonFilter>
func handleFindMany(db *lsm.LSMEngine, rest string) {
	collection, filter, err := parseCollectionAndOneJSON(rest)
	if err != nil {
		fmt.Println("Usage: findMany <collection> <jsonFilter> — parse error:", err)
		return
	}
	keys, _ := db.IterKeys()
	results := 0
	for _, k := range keys {
		if !strings.HasPrefix(k, collection+":") {
			continue
		}
		val, err := db.Get([]byte(k))
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if err := json.Unmarshal(val, &doc); err != nil {
			continue
		}
		if matchFilter(doc, filter) {
			js, _ := json.MarshalIndent(doc, "", "  ")
			fmt.Println(string(js))
			results++
		}
	}
	if results == 0 {
		// print empty array for clarity
		fmt.Println("[]")
	}
}

// updateOne <collection> <jsonFilter> <jsonUpdate>
func handleUpdateOne(db *lsm.LSMEngine, rest string) {
	collection, filter, update, err := parseCollectionAndTwoJSON(rest)
	if err != nil {
		fmt.Println("Usage: updateOne <collection> <filterJSON> <updateJSON> — parse error:", err)
		return
	}
	idVal, ok := filter["_id"]
	if !ok {
		fmt.Println("updateOne currently supports {_id:...}")
		return
	}
	id := fmt.Sprintf("%v", idVal)
	key := collection + ":" + id

	val, err := db.Get([]byte(key))
	if err != nil {
		fmt.Println("Not found:", id)
		return
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(val, &doc); err != nil {
		fmt.Println("Invalid stored document:", err)
		return
	}

	// update may be { "$set": { ... } } - treat update generically
	if setObj, ok := update["$set"]; ok {
		if setMap, ok2 := setObj.(map[string]interface{}); ok2 {
			for k, v := range setMap {
				doc[k] = v
			}
		} else {
			fmt.Println("$set must be an object")
			return
		}
	} else {
		// support direct replacement if top-level not $set
		// if update has "_id" keep same id
		for k, v := range update {
			doc[k] = v
		}
	}

	newRaw, _ := json.Marshal(doc)
	if err := db.Put([]byte(key), newRaw); err != nil {
		fmt.Println("Update error:", err)
		return
	}
	fmt.Printf("Updated %s in %s\n", id, collection)
}

// deleteOne <collection> <jsonFilter>
func handleDeleteOne(db *lsm.LSMEngine, rest string) {
	collection, filter, err := parseCollectionAndOneJSON(rest)
	if err != nil {
		fmt.Println("Usage: deleteOne <collection> <jsonFilter> — parse error:", err)
		return
	}
	idVal, ok := filter["_id"]
	if !ok {
		fmt.Println("deleteOne currently supports {_id:...}")
		return
	}
	id := fmt.Sprintf("%v", idVal)
	if err := db.Delete([]byte(collection + ":" + id)); err != nil {
		fmt.Println("Delete error:", err)
		return
	}
	fmt.Printf("Deleted %s from %s\n", id, collection)
}

// dumpAll <collection>
func handleDumpAll(db *lsm.LSMEngine, rest string) {
	collection := strings.TrimSpace(rest)
	if collection == "" {
		fmt.Println("Usage: dumpAll <collection>")
		return
	}
	keys, _ := db.IterKeys()
	out := []map[string]interface{}{}
	for _, k := range keys {
		if !strings.HasPrefix(k, collection+":") {
			continue
		}
		val, err := db.Get([]byte(k))
		if err != nil {
			continue
		}
		var doc map[string]interface{}
		if json.Unmarshal(val, &doc) == nil {
			out = append(out, doc)
		}
	}
	if len(out) == 0 {
		fmt.Println("[]")
		return
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}

// dumpDB
func handleDumpDB(db *lsm.LSMEngine, parts string) {
	filename := fmt.Sprintf("dump_%s.json", time.Now().Format("15-04_02-01-2006"))
	if err := db.DumpDB(filename); err != nil {
		fmt.Println("Dump error:", err)
		return
	}
	fmt.Println("Dumped DB to", filename)
}

// restoreDB <file.json>
func handleRestoreDB(db *lsm.LSMEngine, rest string) {
	file := strings.TrimSpace(rest)
	if file == "" {
		fmt.Println("Usage: restoreDB <file.json>")
		return
	}
	if _, err := os.Stat(file); os.IsNotExist(err) {
		fmt.Println("File does not exist:", file)
		return
	}
	if err := db.RestoreDB(file); err != nil {
		fmt.Println("Restore error:", err)
		return
	}
	fmt.Println("Restored DB from", file)
}

// compact
func handleCompact(db *lsm.LSMEngine) {
	if err := db.Compact(); err != nil {
		fmt.Println("Compaction error:", err)
		return
	}
	fmt.Println("Compaction done")
}
