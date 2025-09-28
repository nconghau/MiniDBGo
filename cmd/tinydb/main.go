package main

import (
	"fmt"
	"log"

	"github.com/chzyer/readline"
	"github.com/your-username/mini-db-go/internal/lsm"
)

func main() {
	db, err := lsm.OpenLSM("data/MiniDBGo")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(ColorGreen + "MiniDBGo CLI (Mongo-like, @Index21)" + ColorReset)
	fmt.Println("Commands:")
	fmt.Println(ColorCyan + " insertOne, findOne, findMany, updateOne, deleteOne, dumpAll, dumpDB, restoreDB, compact, exit" + ColorReset)
	fmt.Println(ColorYellow + "\nExamples:" + ColorReset)
	fmt.Println(" insertOne customers {\"_id\":\"c1\",\"name\":\"Alice\",\"group\":\"vip\",\"rating\":8}")
	fmt.Println(" findOne customers {\"_id\":\"c1\"}")
	fmt.Println(" findMany customers {\"group\":\"vip\"}")
	fmt.Println(" findMany customers {\"rating\":{\"$gt\":5}}")
	fmt.Println(" updateOne customers {\"_id\":\"c1\"} {\"$set\":{\"name\":\"Alice Updated\"}}")
	fmt.Println(" deleteOne customers {\"_id\":\"c1\"}")
	fmt.Println(" dumpAll customers")
	fmt.Println(" dumpDB                      # export all collections")
	fmt.Println(" restoreDB <file.json>       # restore (will overwrite current db file)")
	fmt.Println(" compact                     # compact DB file (reclaims space)")
	fmt.Println(" exit\n")

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
