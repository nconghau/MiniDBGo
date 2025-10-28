package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nconghau/MiniDBGo/internal/lsm"
)

// insertOne <collection> <jsonDoc>
func handleInsertOne(db *lsm.LSMEngine, rest string) {
	parts := splitArgs(rest, 2)
	if len(parts) < 2 {
		fmt.Println("Usage: insertOne <collection> <jsonDoc>")
		return
	}
	col := parts[0]
	docStr := parts[1]

	var doc map[string]interface{}
	if err := json.Unmarshal([]byte(docStr), &doc); err != nil {
		fmt.Println("Invalid JSON:", err)
		return
	}
	id, ok := doc["_id"].(string)
	if !ok {
		fmt.Println("Document must contain string _id field")
		return
	}

	key := col + ":" + id
	raw, _ := json.Marshal(doc)
	if err := db.Put([]byte(key), raw); err != nil {
		fmt.Println("Insert error:", err)
		return
	}
	fmt.Println("Inserted", id, "into", col)
}

// insertMany <collection> <jsonArrayOfDocs>
func handleInsertMany(db *lsm.LSMEngine, rest string) {
	parts := splitArgs(rest, 2)
	if len(parts) < 2 {
		fmt.Println("Usage: insertMany <collection> <jsonArrayOfDocs>")
		return
	}
	col := parts[0]
	docStr := parts[1]

	var docs []map[string]interface{}
	if err := json.Unmarshal([]byte(docStr), &docs); err != nil {
		fmt.Println("Invalid JSON Array:", err)
		return
	}

	if len(docs) == 0 {
		fmt.Println("No documents to insert.")
		return
	}

	insertedCount := 0
	for i, doc := range docs {
		id, ok := doc["_id"].(string)
		if !ok {
			fmt.Printf("Error at document index %d: Document must contain string _id field\n", i)
			continue // Bỏ qua tài liệu này và tiếp tục
		}

		key := col + ":" + id
		raw, _ := json.Marshal(doc)
		if err := db.Put([]byte(key), raw); err != nil {
			fmt.Printf("Error inserting %s: %v\n", id, err)
			continue // Bỏ qua tài liệu này
		}
		insertedCount++
	}
	fmt.Printf("Inserted %d of %d documents into %s\n", insertedCount, len(docs), col)
}

// findOne <collection> <jsonFilter>
func handleFindOne(db *lsm.LSMEngine, rest string) {
	parts := splitArgs(rest, 2)
	if len(parts) < 2 {
		fmt.Println("Usage: findOne <collection> <jsonFilter>")
		return
	}
	col := parts[0]
	filterStr := parts[1]

	var filter map[string]interface{}
	if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
		fmt.Println("Invalid filter JSON:", err)
		return
	}
	id, ok := filter["_id"].(string)
	if !ok {
		fmt.Println("findOne currently supports {_id:...}")
		return
	}
	key := col + ":" + id
	val, err := db.Get([]byte(key))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println(prettyJSON(val))
}

// findMany <collection> <jsonFilter>
func handleFindMany(db *lsm.LSMEngine, rest string) {
	parts := splitArgs(rest, 2)
	if len(parts) < 2 {
		fmt.Println("Usage: findMany <collection> <jsonFilter>")
		return
	}
	col := parts[0]
	filterStr := parts[1]

	var filter map[string]interface{}
	if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
		fmt.Println("Invalid filter JSON:", err)
		return
	}

	// --- FIX: Use IterKeysWithLimit to prevent OOM ---
	// Use a large limit for CLI, same as server's MaxKeysToReturn
	keys, _ := db.IterKeysWithLimit(10000)

	matchCount := 0
	for _, k := range keys {
		if !strings.HasPrefix(k, col+":") {
			continue
		}

		// --- FIX: Limit number of results returned ---
		if matchCount >= 1000 { // Same limit as server [cite: 41]
			fmt.Println("... (results truncated at 1000)")
			break
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
			fmt.Println(prettyJSON(val))
			matchCount++
		}
	}
}

// updateOne <collection> <jsonFilter> <jsonUpdate>
func handleUpdateOne(db *lsm.LSMEngine, rest string) {
	parts := splitArgs(rest, 3)
	if len(parts) < 3 {
		fmt.Println("Usage: updateOne <collection> <jsonFilter> <jsonUpdate>")
		return
	}
	col := parts[0]
	filterStr := parts[1]
	updateStr := parts[2]

	var filter map[string]interface{}
	if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
		fmt.Println("Invalid filter JSON:", err)
		return
	}
	id, ok := filter["_id"].(string)
	if !ok {
		fmt.Println("updateOne currently supports {_id:...}")
		return
	}
	key := col + ":" + id
	val, err := db.Get([]byte(key))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	var doc map[string]interface{}
	_ = json.Unmarshal(val, &doc)

	var update map[string]map[string]interface{}
	if err := json.Unmarshal([]byte(updateStr), &update); err != nil {
		fmt.Println("Invalid update JSON:", err)
		return
	}
	if set, ok := update["$set"]; ok {
		for k, v := range set {
			doc[k] = v
		}
	}

	raw, _ := json.Marshal(doc)
	if err := db.Put([]byte(key), raw); err != nil {
		fmt.Println("Update error:", err)
		return
	}
	fmt.Println("Updated", id, "in", col)
}

// deleteOne <collection> <jsonFilter>
func handleDeleteOne(db *lsm.LSMEngine, rest string) {
	parts := splitArgs(rest, 2)
	if len(parts) < 2 {
		fmt.Println("Usage: deleteOne <collection> <jsonFilter>")
		return
	}
	col := parts[0]
	filterStr := parts[1]

	var filter map[string]interface{}
	if err := json.Unmarshal([]byte(filterStr), &filter); err != nil {
		fmt.Println("Invalid filter JSON:", err)
		return
	}
	id, ok := filter["_id"].(string)
	if !ok {
		fmt.Println("deleteOne currently supports {_id:...}")
		return
	}
	key := col + ":" + id
	if err := db.Delete([]byte(key)); err != nil {
		fmt.Println("Delete error:", err)
		return
	}
	fmt.Println("Deleted", id, "from", col)
}

// dumpAll <collection>
func handleDumpAll(db *lsm.LSMEngine, rest string) {
	parts := splitArgs(rest, 1)
	if len(parts) < 1 {
		fmt.Println("Usage: dumpAll <collection>")
		return
	}
	col := parts[0]

	// --- FIX: Use IterKeysWithLimit to prevent OOM ---
	keys, _ := db.IterKeysWithLimit(10000) // Use a large limit

	matchCount := 0
	for _, k := range keys {
		if strings.HasPrefix(k, col+":") {
			// --- FIX: Limit number of results returned ---
			if matchCount >= 1000 {
				fmt.Println("... (results truncated at 1000)")
				break
			}

			val, err := db.Get([]byte(k))
			if err == nil {
				fmt.Println(prettyJSON(val))
				matchCount++
			}
		}
	}
}

// dumpDB
func handleDumpDB(db *lsm.LSMEngine, rest string) {
	file := fmt.Sprintf("dump_%s.json", time.Now().Format("150405_02012006"))
	if err := db.DumpDB(file); err != nil {
		fmt.Println("Dump error:", err)
		return
	}
	fmt.Println("Dumped DB to", file)
}

// restoreDB <file.json>
func handleRestoreDB(db *lsm.LSMEngine, rest string) {
	parts := splitArgs(rest, 1)
	if len(parts) < 1 {
		fmt.Println("Usage: restoreDB <file.json>")
		return
	}
	file := parts[0]
	if err := db.RestoreDB(file); err != nil {
		fmt.Println("Restore error:", err)
		return
	}
	fmt.Println("Restored DB from", file)
}

// compact
func handleCompact(db *lsm.LSMEngine) {
	if err := db.Compact(); err != nil {
		fmt.Println("Compact error:", err)
		return
	}
	fmt.Println("Compaction complete")
}

// --- utils ---

func prettyJSON(b []byte) string {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return string(b)
	}
	out, _ := json.MarshalIndent(m, "", "  ")
	return string(out)
}

// splitArgs splits a string into N parts (N-1 splits), keeping JSON intact.
func splitArgs(s string, n int) []string {
	parts := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := strings.IndexAny(s, " \t")
		if idx < 0 {
			return append(parts, strings.TrimSpace(s))
		}
		parts = append(parts, strings.TrimSpace(s[:idx]))
		s = strings.TrimSpace(s[idx+1:])
	}
	if s != "" {
		parts = append(parts, s)
	}
	return parts
}
