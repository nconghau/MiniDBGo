package main

import (
	"fmt"
	"log"
	"os"

	"github.com/chzyer/readline"
	"github.com/nconghau/MiniDBGo/internal/lsm"
)

func main() {
	db, err := lsm.OpenLSM("data/MiniDBGo")
	if err != nil {
		log.Fatal(err)
	}

	go startHttpServer(db, ":6866")

	// Nếu đặt MODE=server, chỉ chạy HTTP server (không CLI)
	if os.Getenv("MODE") == "server" {
		select {} // block mãi mãi
	}

	fmt.Println(ColorYellow + "\nCLI Usage" + ColorReset)
	fmt.Println(ColorCyan + " Commands:" + ColorReset)
	fmt.Println("  insertOne, findOne, findMany, updateOne, deleteOne, dumpAll")

	fmt.Println(ColorCyan + "\n Examples (using 'products' collection):" + ColorReset)
	// insertOne products {"_id":"p1","name":"Laptop","category":"electronics","price":1200}
	fmt.Println("  insertOne products " + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Laptop\"" + ColorReset + "," +
		ColorYellow + "\"category\"" + ColorReset + ":" + ColorCyan + "\"electronics\"" + ColorReset + "," +
		ColorYellow + "\"price\"" + ColorReset + ":" + ColorGreen + "1200" + ColorReset + "}")

	// insertMany products [{"_id":"p1","name":"Laptop"}, {"_id":"p2","name":"Mouse"}]
	fmt.Println("  insertMany products " + ColorReset + "[{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Laptop\"" + ColorReset + "},{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p2\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Mouse\"" + ColorReset + "}]")

	// findOne products {"_id":"p1"}
	fmt.Println("  findOne products " + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "}")

	// findMany products {"category":"electronics"}
	fmt.Println("  findMany products " + ColorReset + "{" +
		ColorYellow + "\"category\"" + ColorReset + ":" + ColorCyan + "\"electronics\"" + ColorReset + "}")

	// findMany products {"price":{"$gt":1000}}
	fmt.Println("  findMany products " + ColorReset + "{" +
		ColorYellow + "\"price\"" + ColorReset + ":{" +
		ColorBlue + "\"$gt\"" + ColorReset + ":" + ColorGreen + "1000" + ColorReset + "}}")

	// updateOne products {"_id":"p1"} {"$set":{"name":"Laptop Pro"}}
	fmt.Println("  updateOne products " + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "} " + ColorReset + "{" +
		ColorBlue + "\"$set\"" + ColorReset + ":{" +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Laptop Pro\"" + ColorReset + "}}")

	// deleteOne products {"_id":"p1"}
	fmt.Println("  deleteOne products " + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "}")

	fmt.Println("  dumpAll products")

	fmt.Println("\n" + ColorCyan + " DB Operations:" + ColorReset)
	fmt.Println("  dumpDB                      " + ColorBlue + "# Export all collections to a file" + ColorReset)
	fmt.Println("  restoreDB <file.json>       " + ColorBlue + "# Restore from a dump file" + ColorReset)
	fmt.Println("  compact                     " + ColorBlue + "# Reclaim space from old data" + ColorReset)
	fmt.Println("  exit")

	// --- Section 2: API ---
	fmt.Println(ColorYellow + "\nREST API Examples (cURL):" + ColorReset)

	fmt.Println(ColorCyan + " # Get all collections" + ColorReset)
	fmt.Println("  curl http://localhost:6866/api/_collections")

	fmt.Println(ColorCyan + " # Get 1 document" + ColorReset)
	fmt.Println("  curl http://localhost:6866/api/products/p1")

	// curl -X PUT -d '{"_id":"p1","name":"Laptop Pro","price":1500}' http://localhost:6866/api/products/p1
	fmt.Println(ColorCyan + " # Create/Update 1 document" + ColorReset)
	fmt.Println("  curl -X PUT -d '" + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Laptop Pro\"" + ColorReset + "," +
		ColorYellow + "\"price\"" + ColorReset + ":" + ColorGreen + "1500" + ColorReset + "}'" +
		" http://localhost:6866/api/products/p1")

	// curl -X POST -d '{"category":"electronics"}' http://localhost:6866/api/products/_search
	fmt.Println(ColorCyan + " # Search documents" + ColorReset)
	fmt.Println("  curl -X POST -d '" + ColorReset + "{" +
		ColorYellow + "\"category\"" + ColorReset + ":" + ColorCyan + "\"electronics\"" + ColorReset + "}'" +
		" http://localhost:6866/api/products/_search")

	// curl -X POST -d '[{"_id":"p2","name":"Mouse"},{"_id":"p3","name":"Keyboard"}]' http://localhost:6866/api/products/_insertMany
	fmt.Println(ColorCyan + " # Insert many documents" + ColorReset)
	fmt.Println("  curl -X POST -d '" + ColorReset + "[{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p2\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Mouse\"" + ColorReset + "},{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p3\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Keyboard\"" + ColorReset + "}]'" +
		" http://localhost:6866/api/products/_insertMany")

	fmt.Println(ColorCyan + " # Delete 1 document" + ColorReset)
	fmt.Println("  curl -X DELETE http://localhost:6866/api/products/p1")

	fmt.Println(ColorCyan + " # Run compaction" + ColorReset)
	fmt.Println("  curl -X POST http://localhost:6866/api/_compact\n")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          ColorYellow + "> " + ColorReset,
		HistoryFile:     "/tmp/MiniDBGo.history",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		AutoComplete:    completer{db: db}, // from autocomplete.go
	})
	if err != nil {
		log.Fatal(err)
	}
	defer rl.Close()

	RunCLI(db, rl)
}
