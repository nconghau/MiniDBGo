# mini-db-go

🚀 A tiny file-based key-value database implemented in Go.  
Goal: Learn database internals by "reinventing the wheel".  

## Features (Phase 1-2)
- Append-only storage (file-based).
- In-memory index for fast lookup.
- Index rebuild on startup (data persistence).
- Simple API: Put, Get, Delete.
- CLI demo.

## Run

```bash
git clone https://github.com/your-username/mini-db-go.git
cd mini-db-go
go run cmd/tinydb/main.go
