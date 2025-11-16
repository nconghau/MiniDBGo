package lsm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

const manifestFileName = "MANIFEST"

// FileMetadata lưu trữ thông tin bền vững về một SSTable
// Nó được thiết kế để dễ dàng serialize/deserialize ra JSON
type FileMetadata struct {
	Level    int    `json:"level"`
	Path     string `json:"path"`
	MinKey   string `json:"minKey"`
	MaxKey   string `json:"maxKey"`
	FileSize int64  `json:"fileSize"`
	KeyCount uint32 `json:"keyCount"`
}

// Version đại diện cho một snapshot (ảnh chụp)
// của trạng thái LSM-Tree
type Version struct {
	// Levels ánh xạ một cấp (level) tới danh sách các tệp trong cấp đó
	// L0: Có thể chồng lấn, sắp xếp theo tệp mới nhất
	// L1+: Không chồng lấn, sắp xếp theo MinKey
	Levels map[int][]*FileMetadata `json:"levels"`
}

// NewVersion tạo một Version rỗng
func NewVersion() *Version {
	return &Version{
		Levels: make(map[int][]*FileMetadata),
	}
}

// AddFile thêm một tệp vào Version
func (v *Version) AddFile(meta *FileMetadata) {
	level := meta.Level
	v.Levels[level] = append(v.Levels[level], meta)

	if level == 0 {
		// L0 sắp xếp theo tệp mới nhất (thêm vào cuối)
	} else {
		// L1+ sắp xếp theo key
		sort.Slice(v.Levels[level], func(i, j int) bool {
			return v.Levels[level][i].MinKey < v.Levels[level][j].MinKey
		})
	}
}

// DeleteFiles xóa các tệp khỏi Version
func (v *Version) DeleteFiles(level int, filesToRemove []*FileMetadata) {
	keep := make([]*FileMetadata, 0, len(v.Levels[level]))
	removeSet := make(map[string]struct{}, len(filesToRemove))
	for _, f := range filesToRemove {
		removeSet[f.Path] = struct{}{}
	}

	for _, f := range v.Levels[level] {
		if _, exists := removeSet[f.Path]; !exists {
			keep = append(keep, f)
		}
	}
	v.Levels[level] = keep
}

// --- Quản lý Manifest ---

// loadManifest đọc tệp MANIFEST và khôi phục Version
func loadManifest(dir string) (*Version, error) {
	path := filepath.Join(dir, manifestFileName)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewVersion(), nil // Không tìm thấy, tạo mới
		}
		return nil, err // Lỗi khác
	}
	defer f.Close()

	var v Version
	if err := json.NewDecoder(f).Decode(&v); err != nil {
		return nil, err
	}
	return &v, nil
}

// saveManifest ghi đè tệp MANIFEST với Version hiện tại
// (Sử dụng kỹ thuật atomic rename)
func (e *LSMEngine) saveManifest() error {
	tempPath := filepath.Join(e.dir, manifestFileName+".tmp")
	f, err := os.Create(tempPath)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ") // Pretty-print để dễ debug

	if err := enc.Encode(e.current); err != nil {
		f.Close()
		os.Remove(tempPath)
		return err
	}

	if err := f.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}

	// Đổi tên (atomic)
	return os.Rename(tempPath, e.manifestPath)
}
