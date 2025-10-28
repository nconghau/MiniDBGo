package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/chzyer/readline"
	"github.com/nconghau/MiniDBGo/internal/lsm"
)

func main() {

	// Set up structured JSON logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Log all levels (Debug, Info, Warn, Error)
	}))
	slog.SetDefault(logger)

	// Set memory limit (optional but recommended)
	if memLimit := os.Getenv("GOMEMLIMIT"); memLimit != "" {
		slog.Info("GOMEMLIMIT set", "value", memLimit)
	}

	// Set GOGC for better memory management
	// Lower values = more frequent GC = less memory usage
	debug.SetGCPercent(50) // Default is 100

	// Limit goroutines
	runtime.GOMAXPROCS(runtime.NumCPU())

	slog.Info("Starting MiniDBGo", "pid", os.Getpid())

	// Open database with production settings
	flushSize := int64(10000)              // 10k records
	maxMemBytes := int64(50 * 1024 * 1024) // 50MB

	// Allow configuration via environment
	if val := os.Getenv("FLUSH_SIZE"); val != "" {
		fmt.Sscanf(val, "%d", &flushSize)
	}
	if val := os.Getenv("MAX_MEM_MB"); val != "" {
		var mb int64
		fmt.Sscanf(val, "%d", &mb)
		maxMemBytes = mb * 1024 * 1024
	}

	db, err := lsm.OpenLSMWithConfig("data/MiniDBGo", flushSize, maxMemBytes)
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer func() {
		slog.Info("Closing database")
		if err := db.Close(); err != nil {
			slog.Error("Database close error", "error", err)
		}
	}()

	// Start HTTP server with graceful shutdown
	server := startHttpServer(db, ":6866")
	_ = server // Keep reference to prevent GC

	// Server-only mode
	if os.Getenv("MODE") == "server" {
		log.Println("[MAIN] Running in server-only mode")
		select {} // Block forever
	}

	// CLI mode
	printUsage()

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          ColorYellow + "> " + ColorReset,
		HistoryFile:     "/tmp/MiniDBGo.history",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		AutoComplete:    completer{db: db},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer rl.Close()

	RunCLI(db, rl)
}

func printUsage() {
	fmt.Println(ColorYellow + "\nMiniDBGo - Production Ready v2.0" + ColorReset)
	fmt.Println(ColorCyan + "\n📊 System Info:" + ColorReset)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("  Go Version: %s\n", runtime.Version())
	fmt.Printf("  NumCPU: %d\n", runtime.NumCPU())
	fmt.Printf("  Memory Allocated: %.2f MB\n", float64(m.Alloc)/1024/1024)

	fmt.Println(ColorYellow + "\n📝 CLI Usage" + ColorReset)
	fmt.Println(ColorCyan + " Commands:" + ColorReset)
	fmt.Println("  insertOne, findOne, findMany, updateOne, deleteOne, dumpAll")

	fmt.Println(ColorCyan + "\n 💡 Examples (using 'products' collection):" + ColorReset)

	fmt.Println("  insertOne products " + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Laptop\"" + ColorReset + "," +
		ColorYellow + "\"category\"" + ColorReset + ":" + ColorCyan + "\"electronics\"" + ColorReset + "," +
		ColorYellow + "\"price\"" + ColorReset + ":" + ColorGreen + "1200" + ColorReset + "}")

	fmt.Println("  insertMany products " + ColorReset + "[{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Laptop\"" + ColorReset + "},{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p2\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Mouse\"" + ColorReset + "}]")

	fmt.Println("  findOne products " + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "}")

	fmt.Println("  findMany products " + ColorReset + "{" +
		ColorYellow + "\"category\"" + ColorReset + ":" + ColorCyan + "\"electronics\"" + ColorReset + "}")

	fmt.Println("  findMany products " + ColorReset + "{" +
		ColorYellow + "\"price\"" + ColorReset + ":{" +
		ColorBlue + "\"$gt\"" + ColorReset + ":" + ColorGreen + "1000" + ColorReset + "}}")

	fmt.Println("  updateOne products " + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "} " + ColorReset + "{" +
		ColorBlue + "\"$set\"" + ColorReset + ":{" +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Laptop Pro\"" + ColorReset + "}}")

	fmt.Println("  deleteOne products " + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "}")

	fmt.Println("  dumpAll products")

	fmt.Println("\n" + ColorCyan + " 🔧 DB Operations:" + ColorReset)
	fmt.Println("  dumpDB                      " + ColorBlue + "# Export all collections to a file" + ColorReset)
	fmt.Println("  restoreDB <file.json>       " + ColorBlue + "# Restore from a dump file" + ColorReset)
	fmt.Println("  compact                     " + ColorBlue + "# Reclaim space from old data" + ColorReset)
	fmt.Println("  exit")

	fmt.Println(ColorYellow + "\n🌐 REST API Examples (cURL):" + ColorReset)

	fmt.Println(ColorCyan + " # Health check" + ColorReset)
	fmt.Println("  curl http://localhost:6866/api/health")

	fmt.Println(ColorCyan + " # Get metrics" + ColorReset)
	fmt.Println("  curl http://localhost:6866/api/metrics")

	fmt.Println(ColorCyan + " # Get all collections" + ColorReset)
	fmt.Println("  curl http://localhost:6866/api/_collections")

	fmt.Println(ColorCyan + " # Get 1 document" + ColorReset)
	fmt.Println("  curl http://localhost:6866/api/products/p1")

	fmt.Println(ColorCyan + " # Create 1 document (InsertOne)" + ColorReset)
	fmt.Println("  curl -X POST -d '" + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Laptop\"" + ColorReset + "," +
		ColorYellow + "\"price\"" + ColorReset + ":" + ColorGreen + "1200" + ColorReset + "}'" +
		" http://localhost:6866/api/products")

	fmt.Println(ColorCyan + " # Update document (Upsert)" + ColorReset)
	fmt.Println("  curl -X PUT -d '" + ColorReset + "{" +
		ColorYellow + "\"_id\"" + ColorReset + ":" + ColorCyan + "\"p1\"" + ColorReset + "," +
		ColorYellow + "\"name\"" + ColorReset + ":" + ColorCyan + "\"Laptop Pro\"" + ColorReset + "," +
		ColorYellow + "\"price\"" + ColorReset + ":" + ColorGreen + "1500" + ColorReset + "}'" +
		" http://localhost:6866/api/products/p1")

	fmt.Println(ColorCyan + " # Search documents" + ColorReset)
	fmt.Println("  curl -X POST -d '" + ColorReset + "{" +
		ColorYellow + "\"category\"" + ColorReset + ":" + ColorCyan + "\"electronics\"" + ColorReset + "}'" +
		" http://localhost:6866/api/products/_search")

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
	fmt.Println("  curl -X POST http://localhost:6866/api/_compact")

	fmt.Println()
}
