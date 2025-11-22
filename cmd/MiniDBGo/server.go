package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nconghau/MiniDBGo/internal/engine"
	"github.com/rs/cors"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	// Server limits
	MaxRequestBodySize = 10 * 1024 * 1024 // 10MB
	MaxConcurrentReq   = 100
	RequestTimeout     = 30 * time.Second
	ShutdownTimeout    = 30 * time.Second
	ReadTimeout        = 15 * time.Second
	WriteTimeout       = 15 * time.Second
	IdleTimeout        = 60 * time.Second

	// Rate limiting
	// MaxKeysToReturn = 10000
)

type Server struct {
	db         engine.Engine
	httpServer *http.Server
	semaphore  chan struct{}
	shutdown   chan os.Signal
	wg         sync.WaitGroup
}

// startHttpServer starts the web server with graceful shutdown
func startHttpServer(db engine.Engine, addr string) *Server {
	s := &Server{
		db:        db,
		semaphore: make(chan struct{}, MaxConcurrentReq),
		shutdown:  make(chan os.Signal, 1),
	}

	mux := http.NewServeMux()

	// API Endpoints with middleware
	mux.HandleFunc("/api/health", s.withMiddleware(s.handleHealthCheck))
	mux.HandleFunc("/api/stats", s.withMiddleware(s.handleGetStats))
	mux.HandleFunc("/api/metrics", s.withMiddleware(s.handleGetMetrics))
	mux.HandleFunc("/api/_collections", s.withMiddleware(s.handleGetCollections))
	mux.HandleFunc("/api/_compact", s.withMiddleware(s.handleCompact))
	mux.HandleFunc("/api/", s.withMiddleware(s.handleApiRoutes))

	// CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	handler := c.Handler(mux)

	s.httpServer = &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    ReadTimeout,
		WriteTimeout:   WriteTimeout,
		IdleTimeout:    IdleTimeout,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	log.Printf("[HTTP] API server starting on %s\n", addr)

	// Start server in goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[HTTP] ERROR: Server failed: %v\n", err)
		}
	}()

	// Setup graceful shutdown
	signal.Notify(s.shutdown, os.Interrupt, syscall.SIGTERM)
	go s.handleShutdown()

	return s
}

func (s *Server) handleShutdown() {
	<-s.shutdown
	log.Println("[HTTP] Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	// Shutdown HTTP server
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("[HTTP] Shutdown error: %v\n", err)
	}

	// Close database
	if err := s.db.Close(); err != nil {
		log.Printf("[DB] Close error: %v\n", err)
	}

	s.wg.Wait()
	log.Println("[HTTP] Server stopped")
	os.Exit(0)
}

// Middleware chain
func (s *Server) withMiddleware(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Request timeout
		ctx, cancel := context.WithTimeout(r.Context(), RequestTimeout)
		defer cancel()
		r = r.WithContext(ctx)

		// Concurrency limiting
		select {
		case s.semaphore <- struct{}{}:
			defer func() { <-s.semaphore }()
		case <-ctx.Done():
			writeError(w, http.StatusServiceUnavailable, "Server too busy")
			return
		}

		// Body size limiting
		r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

		start := time.Now()

		var bodyBytes []byte
		if r.Method == "POST" || r.Method == "PUT" {
			if r.Body != nil {
				// Read all the bytes from the request body
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					// This error triggers if body > MaxRequestBodySize
					writeError(w, http.StatusRequestEntityTooLarge, "Request payload is too large")
					return
				}
				r.Body.Close() // Close the original body

				// Restore the body so the handler can read it
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Run the actual API handler
		handler(w, r)

		// Use slog.LogAttrs for dynamic attributes
		attrs := []slog.Attr{
			slog.String("component", "http"),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		}

		// Only add payload if it's a POST/PUT and we read bytes
		if (r.Method == "POST" || r.Method == "PUT") && len(bodyBytes) > 0 {
			attrs = append(attrs, slog.String("payload", string(bodyBytes)))
		}

		// Log everything together
		slog.LogAttrs(r.Context(), slog.LevelInfo, "HTTP request", attrs...)
	}
}

func (s *Server) handleApiRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "Invalid API path")
		return
	}

	switch {
	case r.Method == "POST" && len(parts) == 2 && parts[1] == "_insertMany":
		s.handleInsertMany(w, r, parts[0])

	case r.Method == "POST" && len(parts) == 2 && parts[1] == "_search":
		s.handleFindMany(w, r, parts[0])

	case r.Method == "POST" && len(parts) == 1:
		s.handleInsertOne(w, r, parts[0])

	case len(parts) == 2:
		collection := parts[0]
		id := parts[1]
		key := []byte(collection + ":" + id)

		switch r.Method {
		case "PUT":
			s.handleUpdateDocument(w, r, key)
		case "GET":
			s.handleGetDocument(w, r, key)
		case "DELETE":
			s.handleDeleteDocument(w, r, key)
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not supported")
		}
	default:
		if !strings.HasSuffix(parts[0], ".ico") {
			writeError(w, http.StatusNotFound, "Invalid API path")
		}
	}
}

type CollectionInfo struct {
	Name     string `json:"name"`
	DocCount int    `json:"docCount"`
	ByteSize int64  `json:"byteSize"`
}

// --- SỬA ĐỔI: Viết lại bằng Iterator ---
func (s *Server) handleGetCollections(w http.ResponseWriter, r *http.Request) {
	colCounts := make(map[string]int)

	it, err := s.db.NewIterator()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create iterator")
		return
	}
	defer it.Close()

	count := 0
	for it.Next() {
		// Giới hạn tổng số key quét (phòng thủ)
		count++
		// if count > MaxKeysToReturn {
		// 	break
		// }

		key := it.Key()
		if idx := strings.Index(key, ":"); idx >= 0 { //
			colName := key[:idx]
			colCounts[colName]++
		}
	}

	if err := it.Error(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed during iteration")
		return
	}

	colsInfo := make([]CollectionInfo, 0, len(colCounts))
	for colName, count := range colCounts {
		colsInfo = append(colsInfo, CollectionInfo{
			Name:     colName,
			DocCount: count,
			ByteSize: 0,
		})
	}

	sort.Slice(colsInfo, func(i, j int) bool {
		return colsInfo[i].Name < colsInfo[j].Name
	})

	writeJSON(w, http.StatusOK, colsInfo)
}

// --- KẾT THÚC SỬA ĐỔI ---

func (s *Server) handleInsertOne(w http.ResponseWriter, r *http.Request, collection string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var doc map[string]interface{}
	if err := json.Unmarshal(body, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "Request body is not valid JSON object")
		return
	}

	id, ok := doc["_id"].(string)
	if !ok {
		writeError(w, http.StatusBadRequest, "Document is missing required _id (string) field")
		return
	}

	key := []byte(collection + ":" + id)

	if err := s.db.Put(key, body); err != nil {
		if strings.Contains(err.Error(), "too many pending flushes") {
			writeError(w, http.StatusServiceUnavailable, "Database is busy, please retry")
			return
		}
		msg := fmt.Sprintf("Error inserting document %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, msg)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"status": "created", "key": string(key)})
}

// handleInsertMany
// --- SỬA ĐỔI: Dùng interface ---
func (s *Server) handleInsertMany(w http.ResponseWriter, r *http.Request, collection string) {
	var docs []map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&docs); err != nil {
		writeError(w, http.StatusBadRequest, "Request body is not a valid JSON array")
		return
	}
	defer r.Body.Close()
	if len(docs) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "insertedCount": 0})
		return
	}
	if len(docs) > 1000 {
		writeError(w, http.StatusBadRequest, "Too many documents (max 1000 per batch)")
		return
	}

	batch := s.db.NewBatch() // Hoạt động vì db là interface

	insertedCount := 0
	for i, doc := range docs {
		id, ok := doc["_id"].(string)
		if !ok {
			msg := fmt.Sprintf("Document at index %d is missing required _id (string) field", i)
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		key := []byte(collection + ":" + id)
		raw, err := json.Marshal(doc)
		if err != nil {
			msg := fmt.Sprintf("Failed to marshal document at index %d: %v", i, err)
			writeError(w, http.StatusInternalServerError, msg)
			return
		}
		batch.Put(key, raw)
		insertedCount++
	}

	if err := s.db.ApplyBatch(batch); err != nil { // Hoạt động vì db là interface
		if strings.Contains(err.Error(), "too many pending flushes") { // [cite: 15]
			writeError(w, http.StatusServiceUnavailable, "Database is busy, please retry")
			return
		}
		msg := fmt.Sprintf("Error inserting batch: %v", err)
		writeError(w, http.StatusInternalServerError, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "insertedCount": insertedCount})
}

func (s *Server) handleUpdateDocument(w http.ResponseWriter, r *http.Request, key []byte) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}
	defer r.Body.Close()

	var doc map[string]interface{}
	if err := json.Unmarshal(body, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "Request body is not valid JSON")
		return
	}

	if err := s.db.Put(key, body); err != nil {
		if strings.Contains(err.Error(), "too many pending flushes") {
			writeError(w, http.StatusServiceUnavailable, "Database is busy, please retry")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "key": string(key)})
}

func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request, key []byte) {
	val, err := s.db.Get(key)
	if err != nil {
		writeError(w, http.StatusNotFound, "Key not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(val)
}

func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request, key []byte) {
	if err := s.db.Delete(key); err != nil {
		if strings.Contains(err.Error(), "too many pending flushes") {
			writeError(w, http.StatusServiceUnavailable, "Database is busy, please retry")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "key": string(key)})
}

// handleFindMany
// --- SỬA ĐỔI: Viết lại hoàn toàn bằng Iterator ---
func (s *Server) handleFindMany(w http.ResponseWriter, r *http.Request, collection string) {
	var filter map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&filter); err != nil { // [cite: 19]
		writeError(w, http.StatusBadRequest, "Invalid JSON filter")
		return
	}
	defer r.Body.Close()

	results := make([]map[string]interface{}, 0, 100)

	it, err := s.db.NewIterator()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create iterator")
		return
	}
	defer it.Close()

	prefix := collection + ":"
	matchCount := 0

	for it.Next() {
		key := it.Key()

		if !strings.HasPrefix(key, prefix) {
			continue
		}

		// Giới hạn kết quả trả về
		if matchCount >= 1000 {
			break
		}

		// Lấy giá trị trực tiếp từ iterator
		val := it.Value().Value

		var doc map[string]interface{}
		if err := json.Unmarshal(val, &doc); err != nil { // [cite: 20]
			continue // Bỏ qua JSON hỏng
		}

		if matchFilter(doc, filter) {
			results = append(results, doc)
			matchCount++
		}
	}

	if err := it.Error(); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed during iteration")
		return
	}

	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleCompact(w http.ResponseWriter, r *http.Request) {
	// Run compaction in background to avoid blocking
	go func() {
		if err := s.db.Compact(); err != nil {
			slog.Info("Compaction started", "trigger", "api")
			if err := s.db.Compact(); err != nil {
				slog.Error("Compaction error", "error", err)
			}
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "compaction started"})
}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.db.GetMetrics()
	writeJSON(w, http.StatusOK, metrics)
}

func getContainerMemoryLimitMB() (float64, error) {
	// Try cgroups v1
	if b, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		s := strings.TrimSpace(string(b))
		val, err := strconv.ParseUint(s, 10, 64)
		if err == nil && val < (1<<60) {
			return float64(val) / 1024 / 1024, nil
		}
	}

	// Try cgroups v2
	if b, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		s := strings.TrimSpace(string(b))
		if s != "max" {
			val, err := strconv.ParseUint(s, 10, 64)
			if err == nil {
				return float64(val) / 1024 / 1024, nil
			}
		}
	}

	// Fallback: Get total host RAM
	v, err := mem.VirtualMemory()
	if err == nil {
		return float64(v.Total) / 1024 / 1024, nil
	}

	return 0, errors.New("could not determine memory limit")
}

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get process info")
		return
	}

	// Use non-blocking CPU measurement with cached values
	cpuPercent, _ := p.CPUPercent()
	memInfo, _ := p.MemoryInfo()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get system CPU without blocking (use interval 0 for instant read)
	totalCpuPercent, _ := cpu.Percent(0, false)

	memLimitMB, _ := getContainerMemoryLimitMB()

	stats := map[string]interface{}{
		"process_cpu_percent":  cpuPercent,
		"process_rss_mb":       memInfo.RSS / 1024 / 1024,
		"process_rss_limit_mb": memLimitMB,
		"go_num_goroutine":     runtime.NumGoroutine(),
		"go_alloc_mb":          m.Alloc / 1024 / 1024,
		"go_sys_mb":            m.Sys / 1024 / 1024,
		"go_heap_alloc_mb":     m.HeapAlloc / 1024 / 1024,
		"go_heap_inuse_mb":     m.HeapInuse / 1024 / 1024,
		"go_num_gc":            m.NumGC,
		"system_cpu_percent":   0.0,
	}

	if len(totalCpuPercent) > 0 {
		stats["system_cpu_percent"] = totalCpuPercent[0]
	}

	writeJSON(w, http.StatusOK, stats)
}

// writeJSON efficiently streams JSON response
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[HTTP] ERROR: Failed to encode JSON response: %v", err)
	}
}

// writeError formats and sends a standard JSON error response
func writeError(w http.ResponseWriter, status int, message string) {
	payload := map[string]interface{}{
		"error":  message,
		"status": status,
	}
	writeJSON(w, status, payload)
}
