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
	"exit",
}

// Autocomplete
type completer struct{}

func (c completer) Do(line []rune, pos int) ([][]rune, int) {
	word := string(line)
	suggestions := [][]rune{}
	for _, cmd := range commands {
		if strings.HasPrefix(strings.ToLower(cmd), strings.ToLower(word)) {
			// ✅ chỉ trả phần còn thiếu, tránh double insert
			suffix := cmd[len(word):]
			if suffix != "" {
				suggestions = append(suggestions, []rune(suffix))
			}
		}
	}
	return suggestions, 0
}

// RunCLI vòng lặp chính
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
