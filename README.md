
-----

# MiniDBGo: A Lightweight Database in Go

## Overview

MiniDBGo is a lightweight, educational database engine written in Go. This project is designed as a learning resource to understand core database internals such as CRUD operations, durability through Write-Ahead Logs (WAL), and the storage architecture of a Log-Structured Merge-Tree (LSM-Tree).

## Core Architecture: How It Works (LSM-Tree)

MiniDBGo is built upon a **Log-Structured Merge-Tree (LSM-Tree)**, an architecture optimized for high write throughput. Here’s a simple breakdown of how it operates:

### 01. The Write Path (Handling New Data ✍️)

When you action insert or update database:

1.  **Safety First (WAL)**: The data is immediately written to a **Write-Ahead Log** (`wal.log`) on disk. This acts as a journal, ensuring that no data is lost even if the database crashes.
2.  **Speed in Memory (MemTable)**: The data is then placed into an in-memory data structure called a **MemTable**, which is a sorted SkipList. Writing to memory is extremely fast.
3.  **Flushing to Disk (SSTable)**: When the MemTable grows to a certain size, it is "frozen" (becoming an *Immutable MemTable*) and its sorted data is flushed to a new, read-only file on disk called an **SSTable** (`.sst` file).

```mermaid
flowchart TD
    subgraph Write Path 
        A[Client Command: insertOne / updateOne] --> B[Write to WAL (wal.log)]
        B --> C[Insert into MemTable (in-memory SkipList)]
        C --> D{MemTable full?}
        D -->|No| C
        D -->|Yes| E[Freeze -> Immutable MemTable]
        E --> F[Flush to Disk as SSTable (.sst)]
        F --> G[Clear WAL for next round]
    end
```

### 02. The Read Path (Finding Your Data 🔍)

When you fetch data MiniDBGo searches for the key in a specific order to ensure the most recent data is found first:

1.  **Check the MemTable**: The active MemTable is checked first, as it contains the very latest writes.
2.  **Check Immutable MemTables**: Any "frozen" MemTables that are waiting to be flushed to disk are checked next.
3.  **Scan SSTables on Disk**: Finally, the database searches the SSTable files on disk, starting from the newest and going to the oldest. A deleted key is marked with a "Tombstone" to indicate it should be ignored.

```mermaid
flowchart TD
    subgraph Read Path 🔍
        H[Client Command: findOne / findMany] --> I[Check MemTable (latest)]
        I -->|Found| Z[✅ Return Result]
        I -->|Not Found| J[Check Immutable MemTables]
        J -->|Found| Z
        J -->|Not Found| K[Search SSTables (newest → oldest)]
        K -->|Found| Z
        K -->|Not Found| L[❌ Key Not Found]
    end
```

### 03. Background Maintenance (Compaction ⚙️)

Over time, many small SSTable files can be created. The `compact` command triggers a **Compaction** process, which merges multiple smaller SSTables into a single, larger one. This process cleans up old or deleted data and optimizes the structure for faster reads.

## ✨ Features

  * **CLI**: An interactive command-line interface with familiar commands.
      * `insertOne`, `findOne`, `findMany`
      * `updateOne` (with `$set` operator), `deleteOne`
      * `dumpDB`, `restoreDB`, `compact`
  * **Durability**: Data safety is ensured with a Write-Ahead Log (WAL).
  * **LSM Storage Engine**: The core is a simple but effective LSM-Tree implementation.
  * **CLI Autocomplete**: Smart command and collection name completion for a better user experience.

## 🚀 Quick Start

```bash
### Terminal 1 ###
git clone https://github.com/nconghau/MiniDBGo

cd MiniDBGo
go run ./cmd/MiniDBGo

### Terminal 2 ###
cd client/MiniDBGoClient
yarn && yarn dev
```

```bash
### CLI Usage ###
Commands:
insertOne, findOne, findMany, updateOne, deleteOne, dumpAll

Examples (using 'products' collection):
insertOne products {"_id":"p1","name":"Laptop","category":"electronics","price":1200}
insertMany products [{"_id":"p1","name":"Laptop"}, {"_id":"p2","name":"Mouse"}]
findOne products {"_id":"p1"}
findMany products {"category":"electronics"}
findMany products {"price":{"$gt":1000}}
updateOne products {"_id":"p1"} {"$set":{"name":"Laptop Pro"}}
deleteOne products {"_id":"p1"}
dumpAll products

### DB Operations: ###
dumpDB          # Export all collections to a file
restoreDB <file.json> # Restore from a dump file
compact         # Reclaim space from old data
exit

### REST API Examples (CURL): ###

# Get all collections
curl http://localhost:6866/api/_collections

# Get 1 document
curl http://localhost:6866/api/products/p1

# Create/Update 1 document
curl -X PUT -d '{"_id":"p1","name":"Laptop Pro","price":1500}' http://localhost:6866/api/products/p1

# Search documents
curl -X POST -d '{"category":"electronics"}' http://localhost:6866/api/products/_search

# Insert many documents
curl -X POST -d '[{"_id":"p2","name":"Mouse"},{"_id":"p3","name":"Keyboard"}]' http://localhost:6866/api/products/_insertMany

# Delete 1 document
curl -X DELETE http://localhost:6866/api/products/p1

# Run compaction
curl -X POST http://localhost:6866/api/_compact
```

## ⚠️ Disclaimer

This project is for **educational purposes only** and is not production-ready. It is intended as a tool for learning database internals in Go.

-----

## 📜 License

MIT License © 2025 [nconghau](https://github.com/nconghau)

