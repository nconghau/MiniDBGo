package lsm

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/nconghau/MiniDBGo/internal/engine"
)

// runCompaction thực hiện logic nén L0 -> L1
func (e *LSMEngine) runL0Compaction(l0Files []*FileMetadata) error {
	// (e.mu.RLock() đã bị comment, đúng rồi)

	if len(l0Files) == 0 {
		return nil // Không có gì để nén
	}

	slog.Info("Starting L0->L1 compaction | runL0Compaction", "files", len(l0Files))

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

	// --- BẮT ĐẦU MÃ TỐI ƯU ---
	keysWritten := 0
	const throttleAfterKeys = 1000 // Nhường CPU sau mỗi 1000 key
	// --- KẾT THÚC MÃ TỐI ƯU ---

	for mergedIter.Next() {
		// MergingIterator đã xử lý tombstones và de-dup
		if err := writer.WriteEntry(mergedIter.Key(), mergedIter.Value()); err != nil {
			writer.Close()
			os.Remove(path)
			return err
		}
		hasEntries = true

		// --- BẮT ĐẦU MÃ TỐI ƯU ---
		keysWritten++
		if keysWritten%throttleAfterKeys == 0 {
			// Yêu cầu Go scheduler chạy các goroutine khác
			// (ví dụ: API handler đang chờ)
			runtime.Gosched()
		}
		// --- KẾT THÚC MÃ TỐI ƯU ---
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

func (e *LSMEngine) runL1Compaction(l1Files, l2Files []*FileMetadata) error {
	if len(l1Files) == 0 {
		return nil // Không có gì để nén
	}

	// 1. Chọn file L1 (chiến lược đơn giản: chọn file cũ nhất)
	l1FileToCompact := l1Files[0]
	filesToCompactL1 := []*FileMetadata{l1FileToCompact}

	minKey := l1FileToCompact.MinKey
	maxKey := l1FileToCompact.MaxKey

	// 2. Tìm các file L2 bị chồng lấn (overlap)
	filesToCompactL2 := make([]*FileMetadata, 0)
	for _, f := range l2Files {
		if f.MaxKey >= minKey && f.MinKey <= maxKey {
			filesToCompactL2 = append(filesToCompactL2, f)
		}
	}

	slog.Debug("L1->L2 Compaction",
		"l1_file", l1FileToCompact.Path,
		"l2_overlap_count", len(filesToCompactL2))

	// 3. Tạo MergingIterator
	iters := make([]engine.Iterator, 0, len(filesToCompactL1)+len(filesToCompactL2))

	// Thêm 1 file L1
	it, err := NewSSTableIterator(l1FileToCompact.Path)
	if err != nil {
		return fmt.Errorf("create L1 iterator: %w", err)
	}
	iters = append(iters, it)

	// Thêm các file L2 chồng lấn
	for _, meta := range filesToCompactL2 {
		it, err := NewSSTableIterator(meta.Path)
		if err != nil {
			for _, it := range iters {
				it.Close()
			}
			return fmt.Errorf("create L2 iterator: %w", err)
		}
		iters = append(iters, it)
	}

	mergedIter := NewMergingIterator(iters)
	defer mergedIter.Close()

	// 4. Tạo file SSTable L2 mới
	e.mu.Lock()
	seq := e.seq
	e.seq++
	e.mu.Unlock()

	path := filepath.Join(e.sstDir, fmt.Sprintf("sst-L2-%06d.sst", seq))
	writer, err := NewSSTWriter(path, 0) // Kích thước không xác định
	if err != nil {
		return err
	}

	// 5. Stream từ iterator (L1+L2) sang writer (L2 mới)
	hasEntries := false

	// --- BẮT ĐẦU MÃ TỐI ƯU (Thêm vào L1) ---
	keysWritten := 0
	const throttleAfterKeys = 1000 // Nhường CPU sau mỗi 1000 key
	// --- KẾT THÚC MÃ TỐI ƯU ---

	for mergedIter.Next() {
		if err := writer.WriteEntry(mergedIter.Key(), mergedIter.Value()); err != nil {
			writer.Close()
			os.Remove(path)
			return err
		}
		hasEntries = true

		// --- BẮT ĐẦU MÃ TỐI ƯU (Thêm vào L1) ---
		keysWritten++
		if keysWritten%throttleAfterKeys == 0 {
			runtime.Gosched()
		}
		// --- KẾT THÚC MÃ TỐI ƯU ---
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

	var newL2Meta *FileMetadata
	if hasEntries {
		meta := writer.GetMetadata()
		newL2Meta = &FileMetadata{
			Level:    2, // Cấp L2 MỚI
			Path:     path,
			MinKey:   meta.MinKey,
			MaxKey:   meta.MaxKey,
			FileSize: meta.FileSize,
			KeyCount: meta.KeyCount,
		}
	}

	// 6. Cập nhật MANIFEST (atomic)
	e.mu.Lock()
	// Xóa 1 file L1 cũ
	e.current.DeleteFiles(1, filesToCompactL1)
	// Xóa các file L2 cũ (bị chồng lấn)
	e.current.DeleteFiles(2, filesToCompactL2)
	// Thêm file L2 mới (nếu có)
	if newL2Meta != nil {
		e.current.AddFile(newL2Meta)
	}
	if err := e.saveManifest(); err != nil {
		e.mu.Unlock()
		slog.Error("CRITICAL: Failed to save manifest after L1 compaction", "error", err)
		return err
	}
	e.mu.Unlock()

	// 7. Xóa các tệp cũ (sau khi MANIFEST đã an toàn)
	for _, meta := range filesToCompactL1 {
		os.Remove(meta.Path)
	}
	for _, meta := range filesToCompactL2 {
		os.Remove(meta.Path)
	}

	e.metrics.compacts.Add(1)
	return nil
}
