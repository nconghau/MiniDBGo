package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/your-username/mini-db-go/internal/engine"
)

func main() {
	db, err := engine.Open("data/tiny.db")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("TinyDB CLI (Mongo-like)")
	fmt.Println("Commands:")
	fmt.Println(` insertOne <collection> {"_id":"1","field":"value"}`)
	fmt.Println(` findOne <collection> {"_id":"1"}`)
	fmt.Println(` updateOne <collection> {"_id":"1"} {"$set":{"field":"new"}}`)
	fmt.Println(` deleteOne <collection> {"_id":"1"}`)
	fmt.Println(" exit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "insertone":
			if len(parts) < 3 {
				fmt.Println("Usage: insertOne <collection> <jsonDoc>")
				continue
			}
			collection := parts[1]
			docRaw := strings.TrimSpace(line[len(cmd)+len(collection)+2:])

			var doc map[string]interface{}
			if err := json.Unmarshal([]byte(docRaw), &doc); err != nil {
				fmt.Println("Invalid JSON:", err)
				continue
			}
			id, ok := doc["_id"].(string)
			if !ok {
				fmt.Println("Document must have string _id field")
				continue
			}
			if err := db.Put([]byte(collection+":"+id), []byte(docRaw)); err != nil {
				fmt.Println("Error:", err)
			} else {
				fmt.Println("Inserted", collection, id)
			}

		case "findone":
			if len(parts) < 3 {
				fmt.Println("Usage: findOne <collection> <jsonQuery>")
				continue
			}
			collection := parts[1]
			queryRaw := strings.TrimSpace(line[len(cmd)+len(collection)+2:])

			var query map[string]interface{}
			if err := json.Unmarshal([]byte(queryRaw), &query); err != nil {
				fmt.Println("Invalid JSON:", err)
				continue
			}
			id, ok := query["_id"].(string)
			if !ok {
				fmt.Println("Query must have string _id")
				continue
			}
			val, err := db.Get([]byte(collection + ":" + id))
			if err != nil {
				fmt.Println("Error:", err)
			} else {
				fmt.Println(string(val))
			}

		case "updateone":
			args := strings.SplitN(line, " ", 4)
			if len(args) < 4 {
				fmt.Println("Usage: updateOne <collection> <jsonQuery> <jsonUpdate>")
				continue
			}
			collection := args[1]
			queryRaw := args[2]
			updateRaw := args[3]

			var query map[string]interface{}
			if err := json.Unmarshal([]byte(queryRaw), &query); err != nil {
				fmt.Println("Invalid JSON query:", err)
				continue
			}
			id, ok := query["_id"].(string)
			if !ok {
				fmt.Println("Query must have string _id")
				continue
			}

			oldVal, err := db.Get([]byte(collection + ":" + id))
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}
			var doc map[string]interface{}
			_ = json.Unmarshal(oldVal, &doc)

			var update map[string]map[string]interface{}
			if err := json.Unmarshal([]byte(updateRaw), &update); err != nil {
				fmt.Println("Invalid JSON update:", err)
				continue
			}
			if setFields, ok := update["$set"]; ok {
				for k, v := range setFields {
					doc[k] = v
				}
			}

			newDoc, _ := json.Marshal(doc)
			if err := db.Update([]byte(collection+":"+id), newDoc); err != nil {
				fmt.Println("Error:", err)
			} else {
				fmt.Println("Updated", collection, id)
			}

		case "deleteone":
			if len(parts) < 3 {
				fmt.Println("Usage: deleteOne <collection> <jsonQuery>")
				continue
			}
			collection := parts[1]
			queryRaw := strings.TrimSpace(line[len(cmd)+len(collection)+2:])

			var query map[string]interface{}
			if err := json.Unmarshal([]byte(queryRaw), &query); err != nil {
				fmt.Println("Invalid JSON:", err)
				continue
			}
			id, ok := query["_id"].(string)
			if !ok {
				fmt.Println("Query must have string _id")
				continue
			}
			if err := db.Delete([]byte(collection + ":" + id)); err != nil {
				fmt.Println("Error:", err)
			} else {
				fmt.Println("Deleted", collection, id)
			}

		case "exit", "quit":
			fmt.Println("Bye!")
			return

		default:
			fmt.Println("Unknown command:", cmd)
		}
	}
}
