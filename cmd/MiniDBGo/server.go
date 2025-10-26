package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log" // Use standard logger instead of fmt for production
	"net/http"
	"sort"
	"strings"

	"github.com/nconghau/MiniDBGo/internal/lsm"
	"github.com/rs/cors"
)

// Server struct holds the DB instance
type Server struct {
	db *lsm.LSMEngine
}

// startHttpServer starts the web server in a goroutine
func startHttpServer(db *lsm.LSMEngine, addr string) {
	s := &Server{db: db}
	mux := http.NewServeMux()

	// --- API Endpoints with /api prefix ---
	mux.HandleFunc("/api/_collections", s.handleGetCollections)
	mux.HandleFunc("/api/_compact", s.handleCompact)
	mux.HandleFunc("/api/", s.handleApiRoutes)

	// --- Cấu hình CORS ---
	c := cors.New(cors.Options{
		// Cho phép origin từ React app của bạn
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	// Bọc mux của bạn bằng handler CORS
	handler := c.Handler(mux) //

	log.Printf("[HTTP] API server running on %s\n", addr)

	go func() {
		// Dùng handler đã bọc CORS thay vì mux
		if err := http.ListenAndServe(addr, handler); err != nil { // <-- SỬA ĐỔI: Dùng 'handler' thay vì 'mux'
			log.Printf("[HTTP] ERROR: Server failed: %v\n", err)
		}
	}()
}

func (s *Server) handleApiRoutes(w http.ResponseWriter, r *http.Request) {
	// Trim the /api prefix from the URL path
	path := strings.TrimPrefix(r.URL.Path, "/api")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "Invalid API path")
		return
	}

	switch {

	// POST /{collection}/_insertMany
	case r.Method == "POST" && len(parts) == 2 && parts[1] == "_insertMany":
		s.handleInsertMany(w, r, parts[0]) // parts[0] is the collection name

	// POST /{collection}/_search
	case r.Method == "POST" && len(parts) == 2 && parts[1] == "_search":
		s.handleFindMany(w, r, parts[0]) // parts[0] is the collection name

	// Handle document routes: /{collection}/{id}
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
		// Ignore favicon requests that might slip through
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

func (s *Server) handleGetCollections(w http.ResponseWriter, r *http.Request) {
	keys, err := s.db.IterKeys()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read keys")
		return
	}

	// Sử dụng map để đếm và tính tổng dung lượng
	colCounts := make(map[string]int)
	colSizes := make(map[string]int64) // Dùng int64 cho kích thước (bytes)

	for _, k := range keys {
		if idx := strings.Index(k, ":"); idx >= 0 {
			colName := k[:idx]

			// 1. Đếm số lượng (như cũ)
			colCounts[colName]++

			// 2. Lấy value và tính dung lượng (PHẦN GÂY CHẬM)
			val, err := s.db.Get([]byte(k))
			if err == nil {
				colSizes[colName] += int64(len(val))
			}
		}
	}

	// Chuyển map thành một slice các struct CollectionInfo
	colsInfo := make([]CollectionInfo, 0, len(colCounts))
	for colName, count := range colCounts {
		colsInfo = append(colsInfo, CollectionInfo{
			Name:     colName,
			DocCount: count,
			ByteSize: colSizes[colName], // Lấy tổng dung lượng từ map
		})
	}

	// Sắp xếp theo tên để API trả về nhất quán
	sort.Slice(colsInfo, func(i, j int) bool {
		return colsInfo[i].Name < colsInfo[j].Name
	})

	writeJSON(w, http.StatusOK, colsInfo)
}

// handleInsertMany inserts multiple documents into a collection
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

		if err := s.db.Put(key, raw); err != nil {
			msg := fmt.Sprintf("Error inserting document %s: %v", id, err)
			writeError(w, http.StatusInternalServerError, msg)
			return
		}
		insertedCount++
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "key": string(key)})
}

func (s *Server) handleFindMany(w http.ResponseWriter, r *http.Request, collection string) {
	var filter map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON filter")
		return
	}
	defer r.Body.Close()

	results := []map[string]interface{}{}
	keys, _ := s.db.IterKeys()
	prefix := collection + ":"

	for _, k := range keys {
		if !strings.HasPrefix(k, prefix) {
			continue
		}

		val, err := s.db.Get([]byte(k))
		if err != nil {
			continue // Document might have been deleted concurrently
		}

		var doc map[string]interface{}
		if err := json.Unmarshal(val, &doc); err != nil {
			continue // Data corruption? Skip this entry.
		}

		if matchFilter(doc, filter) {
			results = append(results, doc)
		}
	}

	writeJSON(w, http.StatusOK, results)
}

func (s *Server) handleCompact(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Compact(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "compaction complete"})
}

// writeJSON streams a JSON response.
// Using json.NewEncoder is more memory-efficient for large responses
// than json.Marshal, as it avoids buffering the entire response in memory.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The header is already sent, so we can't send a clean error payload.
		// We can only log the error.
		log.Printf("[HTTP] ERROR: Failed to encode JSON response: %v", err)
	}
}

// writeError formats and sends a standard JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	payload := map[string]interface{}{
		"error":  message,
		"status": status,
	}
	writeJSON(w, status, payload)
}
