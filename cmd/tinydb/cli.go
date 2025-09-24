package main

import (
	"strings"

	"github.com/chzyer/readline"
	"github.com/your-username/mini-db-go/internal/engine"
)

var commands = []string{
	"insertOne",
	"findOne",
	"findMany",
	"updateOne",
	"deleteOne",
	"dumpAll",
	"dumpDB",
	"restoreDB",
	"compact",
	"exit",
}

// Autocomplete with collection names; completer holds db pointer
type completer struct {
	db *engine.Engine
}

func (c completer) Do(line []rune, pos int) ([][]rune, int) {
	prefix := string(line[:pos])
	trim := strings.TrimLeft(prefix, " ")
	// split only first two tokens
	parts := strings.SplitN(trim, " ", 3)

	// If empty or completing command name
	if len(parts) == 0 || strings.Index(trim, " ") == -1 {
		word := strings.TrimSpace(trim)
		suggestions := [][]rune{}
		for _, cmd := range commands {
			if strings.HasPrefix(strings.ToLower(cmd), strings.ToLower(word)) {
				// return suffix (so we don't duplicate already typed text)
				suffix := cmd[len(word):]
				if suffix != "" {
					suggestions = append(suggestions, []rune(suffix))
				}
			}
		}
		return suggestions, 0
	}

	// If we're here: user typed command and a space — complete collection names for commands that need it
	cmd := strings.ToLower(parts[0])
	// commands that accept collection as second arg
	needCollection := map[string]bool{
		"insertone": true,
		"findone":   true,
		"findmany":  true,
		"updateone": true,
		"deleteone": true,
		"dumpall":   true,
	}
	if !needCollection[cmd] || c.db == nil {
		return nil, 0
	}

	// last token (partial collection)
	var partial string
	if len(parts) >= 2 {
		partial = parts[1]
	} else {
		partial = ""
	}

	// gather collections from db index
	seen := map[string]struct{}{}
	for k := range c.db.Index() {
		if idx := strings.Index(k, ":"); idx >= 0 {
			col := k[:idx]
			if strings.HasPrefix(strings.ToLower(col), strings.ToLower(strings.TrimSpace(partial))) {
				suffix := col[len(strings.TrimSpace(partial)):]
				if suffix != "" {
					seen[suffix] = struct{}{}
				}
			}
		}
	}
	suggestions := [][]rune{}
	for s := range seen {
		suggestions = append(suggestions, []rune(s))
	}
	return suggestions, 0
}

func RunCLI(db *engine.Engine, rl *readline.Instance) {
	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		HandleCommand(db, line)
	}
}
