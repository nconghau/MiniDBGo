package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nconghau/MiniDBGo/internal/lsm"
	"github.com/nconghau/MiniDBGo/internal/storage"
)

// Usage: go run ./cmd/MiniDBGo migrate <old-db-path> <lsm-dir>
func mainMigrate() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: migrate <old-db-path> <lsm-dir>")
		os.Exit(1)
	}
	oldPath := os.Args[2]
	lsmDir := os.Args[3]

	// open old DB file
	dbf, err := storage.OpenDBFile(oldPath)
	if err != nil {
		log.Fatalf("open old db failed: %v", err)
	}

	// open LSM engine
	eng, err := lsm.OpenLSM(lsmDir)
	if err != nil {
		log.Fatalf("open lsm failed: %v", err)
	}

	count := 0
	_ = dbf.IterateEntries(func(offset int64, key, value []byte) error {
		// key here is full key bytes (collection:id)
		if err := eng.Put(key, value); err != nil {
			return err
		}
		count++
		return nil
	})
	fmt.Printf("Migrated ~%d entries into LSM at %s\n", count, lsmDir)
}
