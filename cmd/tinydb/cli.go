package main

import (
	"fmt"
	"strings"

	"github.com/chzyer/readline"
	"github.com/your-username/mini-db-go/internal/lsm"
)

// RunCLI reads lines, split command and rest, dispatch to handlers.
func RunCLI(db *lsm.LSMEngine, rl *readline.Instance) {
	for {
		line, err := rl.Readline()
		if err != nil {
			// readline EOF / Ctrl+C
			fmt.Println()
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		cmd, rest := splitCmdRest(line)
		switch strings.ToLower(cmd) {
		case "insertone":
			handleInsertOne(db, rest)
		case "findone":
			handleFindOne(db, rest)
		case "findmany":
			handleFindMany(db, rest)
		case "updateone":
			handleUpdateOne(db, rest)
		case "deleteone":
			handleDeleteOne(db, rest)
		case "dumpall":
			handleDumpAll(db, rest)
		case "dumpdb":
			handleDumpDB(db, rest)
		case "restoredb":
			handleRestoreDB(db, rest)
		case "compact":
			handleCompact(db)
		case "exit", "quit":
			fmt.Println("Bye!")
			return
		default:
			fmt.Println("Unknown command:", cmd)
		}
	}
}

// splitCmdRest splits the input line into the first word (command)
// and the rest (may contain JSONs with spaces).
func splitCmdRest(line string) (cmd, rest string) {
	// find first whitespace
	for i, r := range line {
		if r == ' ' || r == '\t' {
			return line[:i], strings.TrimSpace(line[i+1:])
		}
	}
	return line, ""
}
