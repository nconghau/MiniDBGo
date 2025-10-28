package main

import (
	"strings"

	"github.com/nconghau/MiniDBGo/internal/lsm"
)

// completer implements readline.AutoCompleter
type completer struct {
	db *lsm.LSMEngine
}

var allCommands = []string{
	"insertOne", "insertMany", "findOne", "findMany", "updateOne", "deleteOne",
	"dumpAll", "dumpDB", "restoreDB", "compact", "exit",
}

// Do is called by chzyer/readline.
// `line` is full buffer as runes, `pos` is cursor position.
// We must compute the current token (based on pos) and return completions plus number of chars to replace.
func (c completer) Do(line []rune, pos int) ([][]rune, int) {
	s := string(line)
	if pos < 0 {
		pos = 0
	}
	if pos > len(s) {
		pos = len(s)
	}
	prefix := s[:pos]

	// fields of the prefix (splits on spaces/tabs)
	fields := strings.Fields(prefix)

	// determine token index and current token string
	var token string
	var tokenIndex int // 0 = command, 1 = collection, >=2 = args
	if len(prefix) == 0 {
		token = ""
		tokenIndex = 0
	} else {
		// if the char before cursor is whitespace -> starting a new token
		if pos > 0 && (prefix[pos-1] == ' ' || prefix[pos-1] == '\t') {
			token = ""
			tokenIndex = len(fields) // new token index
		} else {
			if len(fields) == 0 {
				token = prefix
				tokenIndex = 0
			} else {
				token = fields[len(fields)-1]
				tokenIndex = len(fields) - 1
			}
		}
	}

	// helper to compute replacement length: number of chars in current token
	replaceLen := len(token)

	// decide suggestions based on tokenIndex
	switch tokenIndex {
	case 0:
		// completing command name
		return matchAndExpand(allCommands, token), replaceLen
	case 1:
		// completing collection name — only suggest if command expects a collection
		// determine command name (first token from full line)
		cmdName := ""
		allFields := strings.Fields(s)
		if len(allFields) > 0 {
			cmdName = allFields[0]
		}
		cmdName = strings.ToLower(cmdName)

		// commands that take a collection as 1st arg
		cmdsWithColl := map[string]bool{
			"insertone": true, "insertmany": true, "findone": true, "findmany": true,
			"updateone": true, "deleteone": true, "dumpall": true,
		}
		if !cmdsWithColl[cmdName] {
			return nil, 0
		}

		// --- ⬇️  FIXED: Use memory-safe IterKeysWithLimit ⬇️ ---
		// Use a large limit, same as other CLI commands.
		keys, _ := c.db.IterKeysWithLimit(10000)
		// --- ⬆️  END FIX ⬆️ ---

		colSet := map[string]struct{}{}
		for _, k := range keys {
			if idx := strings.Index(k, ":"); idx >= 0 {
				colSet[k[:idx]] = struct{}{}
			}
		}
		cols := make([]string, 0, len(colSet))
		for col := range colSet {
			cols = append(cols, col)
		}
		return matchAndExpand(cols, token), replaceLen
	default:
		// no completion for later tokens (JSON etc.)
		return nil, 0
	}
}

// matchAndExpand returns completions for options given a prefix.
// - if prefix empty -> return all options
// - case-insensitive matching
// - when exactly 1 match -> return that single completion (to allow expansion)
// length (replaceLen) must be handled by caller
func matchAndExpand(options []string, prefix string) [][]rune {
	if prefix == "" {
		return toRunes(options)
	}
	lpre := strings.ToLower(prefix)
	matches := []string{}
	for _, o := range options {
		if strings.HasPrefix(strings.ToLower(o), lpre) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 0 {
		return nil
	}
	// if unique match, return it so readline can replace token
	if len(matches) == 1 {
		return [][]rune{[]rune(matches[0])}
	}
	// multiple matches -> return list
	return toRunes(matches)
}

func toRunes(strs []string) [][]rune {
	out := make([][]rune, 0, len(strs))
	for _, s := range strs {
		out = append(out, []rune(s))
	}
	return out
}
