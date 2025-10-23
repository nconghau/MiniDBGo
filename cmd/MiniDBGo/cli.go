package main

import (
	"fmt"
	"strings"

	"github.com/chzyer/readline"
	"github.com/nconghau/MiniDBGo/internal/lsm"
)

// RunCLI runs the interactive shell for MiniDBGo.
func RunCLI(db *lsm.LSMEngine, rl *readline.Instance) {
	for {
		line, err := rl.Readline()
		if err != nil {
			// Ctrl+D / Ctrl+C / EOF
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
		case "insertmany":
			handleInsertMany(db, rest)
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

// splitCmdRest extracts the command (first token) and the rest of the line (raw).
func splitCmdRest(line string) (cmd, rest string) {
	for i, r := range line {
		if r == ' ' || r == '\t' {
			return line[:i], strings.TrimSpace(line[i+1:])
		}
	}
	return line, ""
}
