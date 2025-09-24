package main

import (
	"fmt"
	"log"

	"github.com/chzyer/readline"
	"github.com/your-username/mini-db-go/internal/engine"
)

func main() {
	db, err := engine.Open("data/tiny.db")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(ColorGreen + "TinyDB CLI (Mongo-like, @Index11)" + ColorReset)
	fmt.Println("Commands:")
	fmt.Println(ColorCyan + " insertOne, findOne, findMany, updateOne, deleteOne, dumpAll, exit" + ColorReset)
	fmt.Println(ColorYellow + "\nExamples:" + ColorReset)
	fmt.Println(" insertOne customers {\"_id\":\"c1\",\"name\":\"Alice\",\"group\":\"vip\"}")
	fmt.Println(" findOne customers {\"_id\":\"c1\"}")
	fmt.Println(" findMany customers {\"group\":\"vip\"}")
	fmt.Println(" updateOne customers {\"_id\":\"c1\"} {\"$set\":{\"name\":\"Bob\"}}")
	fmt.Println(" deleteOne customers {\"_id\":\"c1\"}")
	fmt.Println(" dumpAll customers\n")

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

	RunCLI(db, rl)
}
