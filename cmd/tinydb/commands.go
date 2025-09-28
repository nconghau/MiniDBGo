package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/your-username/mini-db-go/internal/lsm"
)

func handleCommand(db *lsm.LSMEngine, parts []string) {
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "insertone":
		if len(parts) < 3 {
			fmt.Println("Usage: insertOne <collection> <json>")
			return
		}
		collection := parts[1]
		var doc map[string]interface{}
		if err := json.Unmarshal([]byte(parts[2]), &doc); err != nil {
			fmt.Println("Invalid JSON:", err)
			return
		}
		id := fmt.Sprintf("%v", doc["_id"])
		if id == "" {
			fmt.Println("Missing _id in document")
			return
		}
		raw, _ := json.Marshal(doc)
		if err := db.Put([]byte(collection+":"+id), raw); err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println("Inserted", id)

	case "findone":
		if len(parts) < 3 {
			fmt.Println("Usage: findOne <collection> <json>")
			return
		}
		collection := parts[1]
		var q map[string]interface{}
		json.Unmarshal([]byte(parts[2]), &q)
		id := fmt.Sprintf("%v", q["_id"])
		val, err := db.Get([]byte(collection + ":" + id))
		if err != nil {
			fmt.Println("Not found")
			return
		}
		fmt.Println(string(val))

	case "findmany":
		if len(parts) < 3 {
			fmt.Println("Usage: findMany <collection> <query>")
			return
		}
		collection := parts[1]
		var q map[string]interface{}
		if err := json.Unmarshal([]byte(parts[2]), &q); err != nil {
			fmt.Println("Invalid query JSON")
			return
		}
		keys, _ := db.IterKeys()
		var results []map[string]interface{}
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
				if matchQuery(doc, q) {
					results = append(results, doc)
				}
			}
		}
		out, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(out))

	case "updateone":
		if len(parts) < 4 {
			fmt.Println("Usage: updateOne <collection> <query> <update>")
			return
		}
		collection := parts[1]

		var q map[string]interface{}
		json.Unmarshal([]byte(parts[2]), &q)
		id := fmt.Sprintf("%v", q["_id"])
		raw, err := db.Get([]byte(collection + ":" + id))
		if err != nil {
			fmt.Println("Not found:", id)
			return
		}
		var doc map[string]interface{}
		json.Unmarshal(raw, &doc)

		var upd map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(parts[3]), &upd); err != nil {
			fmt.Println("Invalid update JSON:", err)
			return
		}
		if setFields, ok := upd["$set"]; ok {
			for k, v := range setFields {
				doc[k] = v
			}
		} else {
			fmt.Println("Only $set supported")
			return
		}
		newRaw, _ := json.Marshal(doc)
		if err := db.Put([]byte(collection+":"+id), newRaw); err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println("Updated", id)

	case "deleteone":
		if len(parts) < 3 {
			fmt.Println("Usage: deleteOne <collection> <json>")
			return
		}
		collection := parts[1]
		var q map[string]interface{}
		json.Unmarshal([]byte(parts[2]), &q)
		id := fmt.Sprintf("%v", q["_id"])
		if err := db.Delete([]byte(collection + ":" + id)); err != nil {
			fmt.Println("Delete error:", err)
			return
		}
		fmt.Println("Deleted", id)

	case "dumpall":
		if len(parts) < 2 {
			fmt.Println("Usage: dumpAll <collection>")
			return
		}
		collection := parts[1]
		keys, _ := db.IterKeys()
		var results []map[string]interface{}
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
				results = append(results, doc)
			}
		}
		out, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(out))

	case "dumpdb":
		t := time.Now().Format("15_04_02_01_2006")
		file := fmt.Sprintf("dump_%s.json", t)
		if err := db.DumpDB(file); err != nil {
			fmt.Println("Dump error:", err)
			return
		}
		fmt.Println("Dumped DB to", file)

	case "restoredb":
		if len(parts) < 2 {
			fmt.Println("Usage: restoreDB <file.json>")
			return
		}
		if err := db.RestoreDB(parts[1]); err != nil {
			fmt.Println("Restore error:", err)
			return
		}
		fmt.Println("Restored DB from", parts[1])

	case "compact":
		if err := db.Compact(); err != nil {
			fmt.Println("Compaction error:", err)
			return
		}
		fmt.Println("Compaction done")

	case "exit", "quit":
		fmt.Println("Bye!")
		os.Exit(0)

	default:
		fmt.Println("Unknown command:", parts[0])
	}
}
