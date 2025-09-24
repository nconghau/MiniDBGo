package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/your-username/mini-db-go/internal/engine"
)

// 🎨 ANSI colors
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
)

var commands = []string{
	"insertOne",
	"findOne",
	"updateOne",
	"deleteOne",
	"dumpAll",
	"exit",
}

func main() {
	db, err := engine.Open("data/tiny.db")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ColorGreen + "TinyDB CLI (Mongo-like, history + arrow keys, autocomplete, colored)" + ColorReset)
	fmt.Println("Commands:")
	fmt.Println(ColorCyan + " insertOne, findOne, updateOne, deleteOne, dumpAll, exit" + ColorReset)

	// ✅ autocomplete config
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          ColorYellow + "> " + ColorReset,
		HistoryFile:     "/tmp/tinydb.history",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		AutoComplete:    completer{},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		handleCommand(db, line)
	}
}

// ✅ AutoCompleter cho CLI
type completer struct{}

func (c completer) Do(line []rune, pos int) ([][]rune, int) {
	word := string(line)
	suggestions := [][]rune{}
	for _, cmd := range commands {
		if strings.HasPrefix(strings.ToLower(cmd), strings.ToLower(word)) {
			suggestions = append(suggestions, []rune(cmd))
		}
	}
	return suggestions, 0
}

// ✅ xử lý command
func handleCommand(db *engine.Engine, line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return
	}
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "insertone":
		if len(parts) < 3 {
			fmt.Println(ColorRed + "Usage: insertOne <collection> <jsonDoc>" + ColorReset)
			return
		}
		collection := parts[1]
		docRaw := strings.TrimSpace(line[len(cmd)+len(collection)+2:])

		var doc map[string]interface{}
		if err := json.Unmarshal([]byte(docRaw), &doc); err != nil {
			fmt.Println(ColorRed+"Invalid JSON:"+ColorReset, err)
			return
		}
		id, ok := doc["_id"].(string)
		if !ok {
			fmt.Println(ColorRed + "Document must have string _id field" + ColorReset)
			return
		}
		if err := db.Put([]byte(collection+":"+id), []byte(docRaw)); err != nil {
			fmt.Println(ColorRed+"Error:"+ColorReset, err)
		} else {
			fmt.Println(ColorGreen+"Inserted"+ColorReset,
				ColorYellow+collection+ColorReset,
				ColorCyan+id+ColorReset)
		}

	case "findone":
		if len(parts) < 3 {
			fmt.Println(ColorRed + "Usage: findOne <collection> <jsonQuery>" + ColorReset)
			return
		}
		collection := parts[1]
		queryRaw := strings.TrimSpace(line[len(cmd)+len(collection)+2:])

		var query map[string]interface{}
		if err := json.Unmarshal([]byte(queryRaw), &query); err != nil {
			fmt.Println(ColorRed+"Invalid JSON:"+ColorReset, err)
			return
		}
		id, ok := query["_id"].(string)
		if !ok {
			fmt.Println(ColorRed + "Query must have string _id" + ColorReset)
			return
		}
		val, err := db.Get([]byte(collection + ":" + id))
		if err != nil {
			fmt.Println(ColorRed+"Error:"+ColorReset, err)
		} else {
			var pretty map[string]interface{}
			if err := json.Unmarshal(val, &pretty); err == nil {
				b, _ := json.MarshalIndent(pretty, "", "  ")
				fmt.Println(ColorCyan + string(b) + ColorReset)
			} else {
				fmt.Println(ColorCyan + string(val) + ColorReset)
			}
		}

	case "updateone":
		args := strings.SplitN(line, " ", 4)
		if len(args) < 4 {
			fmt.Println(ColorRed + "Usage: updateOne <collection> <jsonQuery> <jsonUpdate>" + ColorReset)
			return
		}
		collection := args[1]
		queryRaw := args[2]
		updateRaw := args[3]

		var query map[string]interface{}
		if err := json.Unmarshal([]byte(queryRaw), &query); err != nil {
			fmt.Println(ColorRed+"Invalid JSON query:"+ColorReset, err)
			return
		}
		id, ok := query["_id"].(string)
		if !ok {
			fmt.Println(ColorRed + "Query must have string _id" + ColorReset)
			return
		}

		oldVal, err := db.Get([]byte(collection + ":" + id))
		if err != nil {
			fmt.Println(ColorRed+"Error:"+ColorReset, err)
			return
		}
		var doc map[string]interface{}
		_ = json.Unmarshal(oldVal, &doc)

		var update map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(updateRaw), &update); err != nil {
			fmt.Println(ColorRed+"Invalid JSON update:"+ColorReset, err)
			return
		}
		if setFields, ok := update["$set"]; ok {
			for k, v := range setFields {
				doc[k] = v
			}
		}

		newDoc, _ := json.Marshal(doc)
		if err := db.Update([]byte(collection+":"+id), newDoc); err != nil {
			fmt.Println(ColorRed+"Error:"+ColorReset, err)
		} else {
			fmt.Println(ColorGreen+"Updated"+ColorReset,
				ColorYellow+collection+ColorReset,
				ColorCyan+id+ColorReset)
		}

	case "deleteone":
		if len(parts) < 3 {
			fmt.Println(ColorRed + "Usage: deleteOne <collection> <jsonQuery>" + ColorReset)
			return
		}
		collection := parts[1]
		queryRaw := strings.TrimSpace(line[len(cmd)+len(collection)+2:])

		var query map[string]interface{}
		if err := json.Unmarshal([]byte(queryRaw), &query); err != nil {
			fmt.Println(ColorRed+"Invalid JSON:"+ColorReset, err)
			return
		}
		id, ok := query["_id"].(string)
		if !ok {
			fmt.Println(ColorRed + "Query must have string _id" + ColorReset)
			return
		}
		if err := db.Delete([]byte(collection + ":" + id)); err != nil {
			fmt.Println(ColorRed+"Error:"+ColorReset, err)
		} else {
			fmt.Println(ColorGreen+"Deleted"+ColorReset,
				ColorYellow+collection+ColorReset,
				ColorCyan+id+ColorReset)
		}

	case "findmany":
		if len(parts) < 3 {
			fmt.Println(ColorRed + "Usage: findMany <collection> <jsonQuery>" + ColorReset)
			return
		}
		collection := parts[1]
		queryRaw := strings.TrimSpace(line[len(cmd)+len(collection)+2:])

		var query map[string]interface{}
		if err := json.Unmarshal([]byte(queryRaw), &query); err != nil {
			fmt.Println(ColorRed+"Invalid JSON query:"+ColorReset, err)
			return
		}

		results := []map[string]interface{}{}
		for key := range db.Index() {
			if strings.HasPrefix(key, collection+":") {
				val, err := db.Get([]byte(key))
				if err != nil {
					continue
				}
				var doc map[string]interface{}
				if err := json.Unmarshal(val, &doc); err != nil {
					continue
				}

				// match query (tất cả field trong query phải khớp)
				match := true
				for k, v := range query {
					if doc[k] != v {
						match = false
						break
					}
				}
				if match {
					results = append(results, doc)
				}
			}
		}

		if len(results) == 0 {
			fmt.Println(ColorYellow + "No documents found" + ColorReset)
		} else {
			data, _ := json.MarshalIndent(results, "", "  ")
			fmt.Println(ColorCyan + string(data) + ColorReset)
		}

	case "dumpall":
		if len(parts) < 2 {
			fmt.Println(ColorRed + "Usage: dumpAll <collection>" + ColorReset)
			return
		}
		collection := parts[1]
		results := []map[string]interface{}{}
		for key := range db.Index() {
			if strings.HasPrefix(key, collection+":") {
				val, err := db.Get([]byte(key))
				if err != nil {
					continue
				}
				var doc map[string]interface{}
				if err := json.Unmarshal(val, &doc); err == nil {
					results = append(results, doc)
				}
			}
		}

		// ✅ export theo format: collection_dump_hh:MM_dd_mm_yyyy.json
		now := time.Now()
		fileName := fmt.Sprintf("%s_dump_%02d-%02d_%02d-%02d-%04d.json",
			collection,
			now.Hour(), now.Minute(),
			now.Day(), now.Month(), now.Year(),
		)

		data, _ := json.MarshalIndent(results, "", "  ")
		if err := ioutil.WriteFile(fileName, data, 0644); err != nil {
			fmt.Println(ColorRed+"Error writing "+fileName+":"+ColorReset, err)
		} else {
			fmt.Println(ColorGreen + "Exported to " + fileName + ColorReset)
		}

	case "exit", "quit":
		fmt.Println(ColorYellow + "Bye!" + ColorReset)
		os.Exit(0)

	default:
		fmt.Println(ColorRed+"Unknown command:"+ColorReset, cmd)
	}
}
