package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"            // <-- Thêm import này
	"path/filepath" // <-- Thêm import này
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

	// --- BẮT ĐẦU THAY ĐỔI ---

	// Endpoint để phục vụ giao diện UI (file ui.html)
	mux.HandleFunc("/ui", s.handleUI)

	// Endpoint API mới để liệt kê các collection
	mux.HandleFunc("/_collections", s.handleGetCollections)

	// API cho các thao tác dữ liệu
	mux.HandleFunc("/", s.handleRoutes)

	// --- KẾT THÚC THAY ĐỔI ---

	fmt.Printf(ColorGreen+"[HTTP] Máy chủ API đang chạy trên %s"+ColorReset+"\n", addr)
	fmt.Printf(ColorGreen + "[HTTP] Giao diện UI có tại: http://localhost:8080/ui" + ColorReset + "\n") // Thêm dòng này

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			fmt.Printf(ColorRed+"[HTTP] Lỗi máy chủ: %v"+ColorReset+"\n", err)
		}
	}()
}

// --- HANDLER MỚI: Phục vụ tệp UI ---
func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	// Giả sử ui.html nằm cùng thư mục với tệp thực thi (hoặc thư mục cmd/MiniDBGo)
	// Bạn có thể cần điều chỉnh đường dẫn này
	path := filepath.Join("cmd", "MiniDBGo", "ui.html")

	// Đọc tệp
	content, err := os.ReadFile(path)
	if err != nil {
		// Thử một đường dẫn khác nếu chạy từ thư mục gốc
		path = "ui.html" // (Nếu bạn đặt ui.html ở thư mục gốc)
		content, err = os.ReadFile(path)
		if err != nil {
			http.Error(w, "Không thể tìm thấy tệp ui.html", http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

// --- HANDLER MỚI: Lấy danh sách collection ---
func (s *Server) handleGetCollections(w http.ResponseWriter, r *http.Request) {
	keys, err := s.db.IterKeys() // [cite: 34]
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

// handleRoutes phân tích URL và điều hướng đến handler thích hợp
func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		// Chuyển hướng trang gốc đến /ui
		http.Redirect(w, r, "/ui", http.StatusFound)
		return
	}

	// Điều hướng dựa trên số lượng phần tử và phương thức
	switch {
	// POST /_compact
	case r.Method == "POST" && len(parts) == 1 && parts[0] == "_compact":
		s.handleCompact(w, r)

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
		// Bỏ qua các request mà chúng ta không xử lý (vd: /favicon.ico)
		// thay vì trả về lỗi 404
		if !strings.HasSuffix(parts[0], ".ico") {
			writeError(w, http.StatusNotFound, "Đường dẫn không hợp lệ")
		}
	}
}

// (Các hàm handler còn lại: handleUpdateDocument, handleGetDocument,
// handleDeleteDocument, handleFindMany, handleCompact, writeJSON, writeError
// ... giữ nguyên như ở bước trước ...)

// handleUpdateDocument xử lý PUT /{collection}/{id}
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

// handleGetDocument xử lý GET /{collection}/{id}
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

// handleDeleteDocument xử lý DELETE /{collection}/{id}
func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request, key []byte) {
	if err := s.db.Delete(key); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "key": string(key)})
}

// handleFindMany xử lý POST /{collection}/_search
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

// handleCompact xử lý POST /_compact
func (s *Server) handleCompact(w http.ResponseWriter, r *http.Request) {
	if err := s.db.Compact(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "compaction complete"})
}

// --- Các hàm tiện ích HTTP ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
