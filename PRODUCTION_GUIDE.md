# MiniDBGo - Production Deployment Guide

## 🎯 Overview of Production Improvements

### Critical OOM Fixes

1. **Memory-Safe Key Iteration**
   - Added `IterKeysWithLimit()` to prevent loading entire DB into memory
   - Streaming approach for SST file scanning
   - Configurable limits (default: 10,000 keys max)

2. **MemTable Size Limits**
   - Both record count AND byte size limits
   - Default: 10k records OR 50MB (whichever comes first)
   - Configurable via environment variables

3. **Bounded Immutable Tables**
   - Maximum 3 concurrent immutable memtables
   - Backpressure when flush queue is full
   - Prevents unbounded memory growth during high write loads

4. **Request Limiting**
   - Maximum 100 concurrent HTTP requests
   - Request body size limit: 10MB
   - Batch insert limit: 1,000 documents per request
   - Search result limit: 1,000 documents

5. **Graceful Shutdown**
   - Proper signal handling (SIGTERM, SIGINT)
   - Flushes all data before exit
   - Timeout protection (30 seconds)

6. **Background Flush Worker**
   - Dedicated goroutine for async flushing
   - Context-based cancellation
   - Error handling without crashes

7. **HTTP Server Timeouts**
   - Read timeout: 15s
   - Write timeout: 15s
   - Idle timeout: 60s
   - Request timeout: 30s

8. **Non-Blocking Stats**
   - CPU measurements use cached values
   - No 1-second blocking calls in stats endpoint

## 🚀 Quick Start

### Development Mode

```bash
# Set memory limits
export GOMEMLIMIT=200MiB
export FLUSH_SIZE=10000
export MAX_MEM_MB=50

# Run the database
go run ./cmd/MiniDBGo
```

### Production Mode (Docker)

```bash
# Build and run with resource limits
docker-compose up --build -d

# Check logs
docker-compose logs -f minidbgo

# Monitor resources
docker stats minidbgo
```

## 📊 Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MODE` | `cli` | Set to `server` for server-only mode |
| `FLUSH_SIZE` | `10000` | Number of records before memtable flush |
| `MAX_MEM_MB` | `50` | Max memtable size in MB |
| `GOMEMLIMIT` | `200MiB` | Go runtime memory limit |
| `GOGC` | `50` | GC percentage (lower = more frequent GC) |

### Docker Resource Limits

```yaml
deploy:
  resources:
    limits:
      cpus: '2'
      memory: 256M
    reservations:
      cpus: '0.5'
      memory: 128M
```

## 🔍 Monitoring

### Health Check

```bash
curl http://localhost:6866/api/health
```

### Metrics Endpoint

```bash
curl http://localhost:6866/api/metrics
```

Response:
```json
{
  "puts": 1234,
  "gets": 5678,
  "deletes": 90,
  "flushes": 12,
  "compacts": 2
}
```

### System Stats

```bash
curl http://localhost:6866/api/stats
```

Response:
```json
{
  "process_cpu_percent": 2.5,
  "process_rss_mb": 85,
  "process_rss_limit_mb": 256,
  "go_num_goroutine": 12,
  "go_alloc_mb": 45,
  "go_sys_mb": 72,
  "go_heap_alloc_mb": 43,
  "go_heap_inuse_mb": 48,
  "go_num_gc": 156,
  "system_cpu_percent": 15.3
}
```

## ⚠️ Production Warnings

### When OOM Will Still Occur

1. **Very Large Single Documents**
   - Each document must fit in memory
   - Limit: 10MB per request body
   - Solution: Split large documents

2. **Extremely High Write Rate**
   - If writes exceed flush rate for extended period
   - Backpressure will return 503 errors
   - Solution: Implement client-side retry with exponential backoff

3. **Large Compaction**
   - Compaction loads all SSTables into memory
   - Happens in background but still uses memory
   - Solution: Run compaction during low-traffic periods

### Best Practices

1. **Regular Compaction**
   ```bash
   # Schedule daily compaction
   curl -X POST http://localhost:6866/api/_compact
   ```

2. **Monitor Memory Usage**
   ```bash
   # Watch memory continuously
   watch -n 5 'docker stats minidbgo --no-stream'
   ```

3. **Backup Strategy**
   ```bash
   # Daily backup
   curl http://localhost:6866/api/dumpDB
   ```

4. **Log Rotation**
   - Configured in docker-compose.yml
   - Max size: 10MB per file
   - Max files: 3

5. **Graceful Restart**
   ```bash
   # Sends SIGTERM for graceful shutdown
   docker-compose restart minidbgo
   ```

## 🔧 Tuning for Your Workload

### High Write Load

```bash
export FLUSH_SIZE=5000      # Flush more frequently
export MAX_MEM_MB=30        # Smaller memtables
export GOGC=30              # More aggressive GC
```

### High Read Load

```bash
export FLUSH_SIZE=20000     # Larger memtables
export MAX_MEM_MB=100       # More memory for cache
export GOGC=100             # Less frequent GC
```

### Memory Constrained

```bash
export FLUSH_SIZE=5000
export MAX_MEM_MB=25
export GOMEMLIMIT=128MiB
export GOGC=30
```

### High Throughput

```bash
export FLUSH_SIZE=50000
export MAX_MEM_MB=200
export GOMEMLIMIT=512MiB
export GOGC=100
```

## 🐛 Troubleshooting

### OOM Killed

```bash
# Check if container was OOM killed
docker inspect minidbgo | grep OOMKilled

# View kernel logs
dmesg | grep -i "out of memory"

# Solution: Reduce memory usage or increase limits
export MAX_MEM_MB=25
export GOMEMLIMIT=150MiB
```

### Slow Performance

```bash
# Check if too many SST files
ls -la data/MiniDBGo/sst/ | wc -l

# Run compaction
curl -X POST http://localhost:6866/api/_compact

# Check GC pressure
curl http://localhost:6866/api/stats | jq '.go_num_gc'
```

### 503 Errors (Database Busy)

```bash
# Check immutable table count (should be < 3)
# Implement retry logic in client:
curl --retry 3 --retry-delay 1 \
  -X POST -d '{"_id":"p1","name":"test"}' \
  http://localhost:6866/api/products
```

### High Memory Usage

```bash
# Force GC (for debugging only)
curl -X POST http://localhost:6866/api/debug/gc

# Check goroutine leaks
curl http://localhost:6866/api/stats | jq '.go_num_goroutine'
# Should be < 50 normally
```

## 📈 Load Testing

### Simple Load Test

```bash
# Install hey: https://github.com/rakyll/hey
go install github.com/rakyll/hey@latest

# Test write load
hey -n 10000 -c 50 -m POST \
  -H "Content-Type: application/json" \
  -d '{"_id":"test-${RANDOM}","data":"test"}' \
  http://localhost:6866/api/products

# Test read load
hey -n 10000 -c 50 \
  http://localhost:6866/api/products/p1
```

### Expected Performance

- **Writes**: 2,000-5,000 ops/sec
- **Reads**: 10,000-20,000 ops/sec
- **Memory**: 50-150 MB under load
- **CPU**: 20-50% on 2 cores

## 🔐 Security Considerations

1. **No Authentication** - Add reverse proxy with auth
2. **No TLS** - Use nginx/caddy for HTTPS
3. **No Input Validation** - Validate on client side
4. **File System Access** - Run as non-root user (✅ implemented)

### Example Nginx Proxy

```nginx
server {
    listen 443 ssl;
    server_name db.example.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location /api/ {
        proxy_pass http://localhost:6866;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        
        # Rate limiting
        limit_req zone=api burst=20 nodelay;
    }
}
```

## 📝 Migration from Old Version

```bash
# 1. Backup old data
curl http://old-server:6866/api/dumpDB > backup.json

# 2. Stop old server
docker-compose down

# 3. Deploy new version
docker-compose up --build -d

# 4. Restore data
curl -X POST -d @backup.json \
  http://localhost:6866/api/restoreDB

# 5. Verify
curl http://localhost:6866/api/_collections
```

## 🎓 Understanding the Improvements

### Before (v1)

```go
// ❌ Loads ALL keys into memory
func (e *LSMEngine) IterKeys() []string {
    keys := []string{}
    // Scans entire database
    for _, k := range allKeys {
        keys = append(keys, k)
    }
    return keys // Can be GBs of data!
}
```

### After (v2)

```go
// ✅ Streams keys with limit
func (e *LSMEngine) IterKeysWithLimit(limit int) []string {
    keys := make(map[string]struct{})
    count := 0
    // Stops early when limit reached
    for _, k := range streamKeys() {
        if limit > 0 && count >= limit {
            break
        }
        keys[k] = struct{}{}
        count++
    }
    return mapToSlice(keys)
}
```

## 📚 Additional Resources

- [Go Memory Management](https://tip.golang.org/doc/gc-guide)
- [Docker Memory Limits](https://docs.docker.com/config/containers/resource_constraints/)
- [LSM Tree Architecture](https://en.wikipedia.org/wiki/Log-structured_merge-tree)

## 🤝 Support

For production issues:
1. Check logs: `docker-compose logs minidbgo`
2. Check metrics: `curl localhost:6866/api/stats`
3. Review this guide
4. Open GitHub issue with logs and metrics