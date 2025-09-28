package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/your-username/mini-db-go/internal/lsm"
)

func RunCLI(db *lsm.LSMEngine) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(ColorYellow + "> " + ColorReset)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 4)
		handleCommand(db, parts)
	}
}
