# TinyDB (Mongo-like) - @Index20

TinyDB is a lightweight, educational database engine written in **Go**, inspired by MongoDB.
This project is designed as a learning resource to understand **database internals** such as CRUD operations, Write-Ahead Logs (WAL), and the foundations of Log-Structured Merge Trees (LSM Trees).

---

## ✨ Features (Index20)

* **Mongo-like CLI** with commands:

  * `insertOne` – Insert a single JSON document
  * `findOne` – Find a single document by filter
  * `findMany` – Find multiple documents with filter
  * `updateOne` – Update a document by filter (`$set` operator supported)
  * `deleteOne` – Delete a document by filter
  * `dumpAll` – Export a collection to JSON
  * `dumpDB` – Export the entire database to JSON
  * `restoreDB` – Restore database from JSON file
  * `compact` – Placeholder for space compaction
  * `exit` – Exit CLI
* **Durability**: Basic Write-Ahead Log (WAL) support.
* **Persistence**: Data stored in JSON + SSTable files.
* **CLI Autocomplete** for commands and collections.
* **Colorized Output** for better UX.

---

## 🚀 Quick Start

### 1. Clone and build

```bash
git clone https://github.com/your-username/tinydb
cd tinydb
go run ./cmd/tinydb
```

### 2. CLI Example Usage

```bash
TinyDB CLI (Mongo-like, @Index20)
Commands: insertOne, findOne, findMany, updateOne, deleteOne, dumpAll, dumpDB, restoreDB, compact, exit

Examples:
 insertOne customers {"_id":"c1","name":"Alice","group":"vip","rating":8}
 findOne customers {"_id":"c1"}
 findMany customers {"group":"vip"}
 findMany customers {"rating":{"$gt":5}}
 updateOne customers {"_id":"c1"} {"$set":{"name":"Alice Updated"}}
 deleteOne customers {"_id":"c1"}
 dumpAll customers
 dumpDB
 restoreDB dump_12_30_09_28_2025.json
 compact
 exit
```

---

## 📂 Project Structure

```
cmd/tinydb/       # CLI entrypoint
  ├── main.go     # Main startup file
  ├── cli.go      # CLI loop (readline, dispatch commands)
  ├── commands.go # CRUD command handlers
  ├── colors.go   # ANSI color helpers
  └── migrate.go  # Future migrations

internal/lsm/     # LSM-tree engine (experimental, Index17+)
internal/engine/  # High-level DB wrapper
```

---

## 🔍 Design Notes

* Data is stored as `collection:id -> JSON`.
* `updateOne` uses `$set` operator for partial updates.
* Deletes are stored as tombstones in memory/disk until **compaction**.
* CLI is built using [readline](https://pkg.go.dev/github.com/chzyer/readline) with autocomplete support.

---

## 📖 Roadmap

* **@Index21**: Add query operators (`$gt`, `$lt`, `$in`, etc.)
* **@Index22**: Bloom filter & sparse index for faster lookups
* **@Index23**: Background compaction & leveled storage
* **@Index24**: Transactions & concurrency
* **@Index25**: HTTP API layer for external apps

---

## ⚠️ Disclaimer

This project is **not production-ready**.
It is built for **educational purposes** to learn database internals in Go.
Contributions and feedback are welcome! 🚀

---

## 📜 License

MIT License © 2025 Your Name