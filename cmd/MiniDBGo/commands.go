package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nconghau/MiniDBGo/internal/engine"
)

// insertOne <collection> <jsonDoc>
func handleInsertOne(db engine.Engine, rest string) {
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
func handleInsertMany(db engine.Engine, rest string) {
	parts := splitArgs(rest, 2)
	if len(parts) < 2 {
		fmt.Println("Usage: insertMany <collection> <jsonArrayOfDocs>")
		return
	}
	col := parts[0]
	docStr := parts[1]

	var docs []map[string]interface{}
	if err := json.Unmarshal([]byte(docStr), &docs); err != nil { // [cite: 40]
		fmt.Println("Invalid JSON Array:", err)
		return
	}

	if len(docs) == 0 {
		fmt.Println("No documents to insert.")
		return
	}

	// --- BẮT ĐẦU MÃ MỚI ---
	batch := db.NewBatch()
	// --- KẾT THÚC MÃ MỚI ---

	insertedCount := 0
	for i, doc := range docs {
		id, ok := doc["_id"].(string)
		if !ok {
			fmt.Printf("Error at document index %d: Document must contain string _id field\n", i)
			continue // Bỏ qua tài liệu này và tiếp tục
		}

		key := col + ":" + id
		raw, _ := json.Marshal(doc)

		// --- SỬA ĐỔI: Thêm vào batch ---
		batch.Put([]byte(key), raw)
		// Logic db.Put() cũ [cite: 41] đã bị xóa
		// --- KẾT THÚC SỬA ĐỔI ---
		insertedCount++
	}

	// --- BẮT ĐẦU MÃ MỚI ---
	// Áp dụng batch
	if err := db.ApplyBatch(batch); err != nil {
		fmt.Printf("Error inserting batch: %v\n", err)
		// Không in ra số lượng, vì batch đã thất bại
		return
	}
	// --- KẾT THÚC MÃ MỚI ---

	fmt.Printf("Inserted %d of %d documents into %s\n", insertedCount, len(docs), col)
}

// findOne <collection> <jsonFilter>
func handleFindOne(db engine.Engine, rest string) {
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
func handleFindMany(db engine.Engine, rest string) {
	parts := splitArgs(rest, 2)
	if len(parts) < 2 {
		fmt.Println("Usage: findMany <collection> <jsonFilter>")
		return
	}
	col := parts[0]
	filterStr := parts[1]

	var filter map[string]interface{}
	if err := json.Unmarshal([]byte(filterStr), &filter); err != nil { // [cite: 43]
		fmt.Println("Invalid filter JSON:", err)
		return
	}

	it, err := db.NewIterator()
	if err != nil {
		fmt.Println("Iterator error:", err)
		return
	}
	defer it.Close()

	matchCount := 0
	prefix := col + ":"

	for it.Next() {
		key := it.Key()
		if !strings.HasPrefix(key, prefix) {
			continue
		}

		if matchCount >= 1000 { // Giới hạn như cũ
			fmt.Println("... (results truncated at 1000)")
			break
		}

		val := it.Value().Value
		var doc map[string]interface{}
		if err := json.Unmarshal(val, &doc); err != nil { // [cite: 44]
			continue
		}

		if matchFilter(doc, filter) {
			fmt.Println(prettyJSON(val))
			matchCount++
		}
	}

	if err := it.Error(); err != nil {
		fmt.Println("Iterator error:", err)
	}
}

// updateOne <collection> <jsonFilter> <jsonUpdate>
func handleUpdateOne(db engine.Engine, rest string) {
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
func handleDeleteOne(db engine.Engine, rest string) {
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

// handleDumpAll
// --- SỬA ĐỔI: Viết lại hoàn toàn bằng Iterator ---
func handleDumpAll(db engine.Engine, rest string) { //
	parts := splitArgs(rest, 1)
	if len(parts) < 1 {
		fmt.Println("Usage: dumpAll <collection>")
		return
	}
	col := parts[0]

	it, err := db.NewIterator()
	if err != nil {
		fmt.Println("Iterator error:", err)
		return
	}
	defer it.Close()

	// Logic OOM cũ dùng IterKeysWithLimit bị xóa

	matchCount := 0
	prefix := col + ":"

	for it.Next() {
		if strings.HasPrefix(it.Key(), prefix) {
			if matchCount >= 1000 {
				fmt.Println("... (results truncated at 1000)")
				break
			}

			val := it.Value().Value
			fmt.Println(prettyJSON(val))
			matchCount++
		}
	}

	if err := it.Error(); err != nil {
		fmt.Println("Iterator error:", err)
	}
}

// --- KẾT THÚC SỬA ĐỔI ---

// dumpDB
func handleDumpDB(db engine.Engine, rest string) {
	file := fmt.Sprintf("dump_%s.json", time.Now().Format("150405_02012006"))
	if err := db.DumpDB(file); err != nil {
		fmt.Println("Dump error:", err)
		return
	}
	fmt.Println("Dumped DB to", file)
}

// restoreDB <file.json>
func handleRestoreDB(db engine.Engine, rest string) {
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
func handleCompact(db engine.Engine) {
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
