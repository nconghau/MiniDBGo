package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nconghau/MiniDBGo/internal/lsm"
)

// Server struct để giữ đối tượng DB
type Server struct {
	db *lsm.LSMEngine
}

// startHttpServer khởi chạy máy chủ web trong một goroutine
func startHttpServer(db *lsm.LSMEngine, addr string) {
	s := &Server{db: db}
	mux := http.NewServeMux()

	// --- THAY ĐỔI: API Endpoints với prefix /api ---

	// Endpoint API mới để liệt kê các collection
	mux.HandleFunc("/api/_collections", s.handleGetCollections)

	// Endpoint API để compact
	mux.HandleFunc("/api/_compact", s.handleCompact)

	// API cho các thao tác dữ liệu (sẽ được xử lý bởi handleApiRoutes)
	// /api/{collection}/{id}
	// /api/{collection}/_search
	// /api/{collection}/_insertMany
	mux.HandleFunc("/api/", s.handleApiRoutes)

	fmt.Printf(ColorGreen+"[HTTP] Máy chủ API đang chạy trên %s"+ColorReset+"\n", addr)
	// fmt.Printf(ColorGreen + "[HTTP] Giao diện UI có tại: http://localhost:6866/" + ColorReset + "\n") // Đã cập nhật

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			fmt.Printf(ColorRed+"[HTTP] Lỗi máy chủ: %v"+ColorReset+"\n", err)
		}
	}()
}

func (s *Server) handleGetCollections(w http.ResponseWriter, r *http.Request) {
	keys, err := s.db.IterKeys()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Không thể đọc keys")
		return
	}
	// Sử dụng map để đảm bảo tên collection là duy nhất
	colSet := map[string]struct{}{}
	for _, k := range keys {
		if idx := strings.Index(k, ":"); idx >= 0 {
			colSet[k[:idx]] = struct{}{}
		}
	}

	// Chuyển map keys thành một slice
	cols := make([]string, 0, len(colSet))
	for col := range colSet {
		cols = append(cols, col)
	}

	writeJSON(w, http.StatusOK, cols)
}

func (s *Server) handleApiRoutes(w http.ResponseWriter, r *http.Request) {
	// --- THAY ĐỔI: Loại bỏ prefix /api/ khỏi URL path ---
	path := strings.TrimPrefix(r.URL.Path, "/api")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "Đường dẫn API không hợp lệ")
		return
	}

	switch {

	// POST /{collection}/_insertMany
	case r.Method == "POST" && len(parts) == 2 && parts[1] == "_insertMany":
		s.handleInsertMany(w, r, parts[0]) // parts[0] là collection

	// POST /{collection}/_search
	case r.Method == "POST" && len(parts) == 2 && parts[1] == "_search":
		s.handleFindMany(w, r, parts[0]) // parts[0] là collection

	// Xử lý tài liệu: /{collection}/{id}
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
			writeError(w, http.StatusMethodNotAllowed, "Phương thức không được hỗ trợ")
		}
	default:
		if !strings.HasSuffix(parts[0], ".ico") {
			writeError(w, http.StatusNotFound, "Đường dẫn API không hợp lệ")
		}
	}
}

// handleInsertMany (Giữ nguyên)
func (s *Server) handleInsertMany(w http.ResponseWriter, r *http.Request, collection string) {
	var docs []map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&docs); err != nil {
		writeError(w, http.StatusBadRequest, "Nội dung không phải là một mảng JSON hợp lệ")
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
			msg := fmt.Sprintf("Tài liệu tại chỉ mục %d thiếu trường _id (kiểu string)", i)
			writeError(w, http.StatusBadRequest, msg)
			return
		}

		key := []byte(collection + ":" + id)
		raw, err := json.Marshal(doc)
		if err != nil {
			msg := fmt.Sprintf("Không thể marshal tài liệu tại chỉ mục %d: %v", i, err)
			writeError(w, http.StatusInternalServerError, msg)
			return
		}

		if err := s.db.Put(key, raw); err != nil {
			msg := fmt.Sprintf("Lỗi khi chèn tài liệu %s: %v", id, err)
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
		writeError(w, http.StatusBadRequest, "Không thể đọc nội dung request")
		return
	}
	defer r.Body.Close()

	var doc map[string]interface{}
	if err := json.Unmarshal(body, &doc); err != nil {
		writeError(w, http.StatusBadRequest, "Nội dung không phải là JSON hợp lệ")
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
		writeError(w, http.StatusNotFound, "Không tìm thấy khóa")
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
		writeError(w, http.StatusBadRequest, "JSON filter không hợp lệ")
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
			continue
		}

		var doc map[string]interface{}
		if err := json.Unmarshal(val, &doc); err != nil {
			continue
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

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if b, err := json.MarshalIndent(v, "", "  "); err == nil {
		_, _ = w.Write(b)
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	payload := map[string]interface{}{
		"error":  message,
		"status": status,
	}
	writeJSON(w, status, payload)
}
