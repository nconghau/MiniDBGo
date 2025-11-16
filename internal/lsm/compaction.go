package lsm

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/nconghau/MiniDBGo/internal/engine"
)

// runCompaction thực hiện logic nén L0 -> L1
// Nó được gọi bởi compactionWorker và nắm giữ compactMu
func (e *LSMEngine) runCompaction() error {
	e.mu.RLock()
	l0Files := make([]*FileMetadata, len(e.current.Levels[0]))
	copy(l0Files, e.current.Levels[0])
	e.mu.RUnlock()

	if len(l0Files) == 0 {
		return nil // Không có gì để nén
	}

	slog.Info("Starting L0->L1 compaction", "files", len(l0Files))

	// 1. Tạo MergingIterator cho TẤT CẢ các tệp L0
	iters := make([]engine.Iterator, 0, len(l0Files))
	for _, meta := range l0Files {
		it, err := NewSSTableIterator(meta.Path)
		if err != nil {
			for _, it := range iters {
				it.Close()
			}
			return fmt.Errorf("create compaction iterator: %w", err)
		}
		iters = append(iters, it)
	}
	mergedIter := NewMergingIterator(iters)
	defer mergedIter.Close()

	// 2. Tạo SSTable L1 mới
	e.mu.Lock()
	seq := e.seq
	e.seq++
	e.mu.Unlock()

	path := filepath.Join(e.sstDir, fmt.Sprintf("sst-L1-%06d.sst", seq))
	writer, err := NewSSTWriter(path, 0) // Kích thước không xác định
	if err != nil {
		return err
	}

	// 3. Stream từ iterator (L0) sang writer (L1)
	hasEntries := false
	for mergedIter.Next() {
		// MergingIterator đã xử lý tombstones và de-dup
		if err := writer.WriteEntry(mergedIter.Key(), mergedIter.Value()); err != nil {
			writer.Close()
			os.Remove(path)
			return err
		}
		hasEntries = true
	}
	if err := mergedIter.Error(); err != nil {
		writer.Close()
		os.Remove(path)
		return err
	}
	if err := writer.Close(); err != nil {
		os.Remove(path)
		return err
	}

	var newL1Meta *FileMetadata
	if hasEntries {
		meta := writer.GetMetadata()
		newL1Meta = &FileMetadata{
			Level:    1, // Cấp L1
			Path:     path,
			MinKey:   meta.MinKey,
			MaxKey:   meta.MaxKey,
			FileSize: meta.FileSize,
			KeyCount: meta.KeyCount,
		}
	}

	// 4. Cập nhật MANIFEST (atomic)
	e.mu.Lock()
	// Xóa tệp L0 cũ
	e.current.DeleteFiles(0, l0Files)
	// Thêm tệp L1 mới (nếu có)
	if newL1Meta != nil {
		e.current.AddFile(newL1Meta)
	}
	// Lưu trạng thái mới
	if err := e.saveManifest(); err != nil {
		e.mu.Unlock()
		slog.Error("CRITICAL: Failed to save manifest after compaction", "error", err)
		return err
	}
	e.mu.Unlock()

	// 5. Xóa các tệp L0 cũ (sau khi MANIFEST đã an toàn)
	for _, meta := range l0Files {
		if err := os.Remove(meta.Path); err != nil {
			slog.Warn("Failed to delete old L0 file after compaction", "path", meta.Path, "error", err)
		}
	}

	e.metrics.compacts.Add(1)
	return nil
}
