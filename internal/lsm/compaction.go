package lsm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Compact merges all SST files at level 0 into one bigger SST
// --- SỬA ĐỔI: Viết lại hoàn toàn bằng MergingIterator ---
func (e *LSMEngine) Compact() error {
	files, err := os.ReadDir(e.sstDir)
	if err != nil {
		return err
	}

	var ssts []string
	for _, fi := range files {
		if strings.HasSuffix(fi.Name(), ".sst") {
			ssts = append(ssts, filepath.Join(e.sstDir, fi.Name())) //
		}
	}
	if len(ssts) <= 1 {
		return nil // nothing to compact
	}
	// Sắp xếp ssts (mặc dù MergingIterator không yêu cầu,
	// nhưng việc này tốt cho việc theo dõi)
	sort.Strings(ssts)

	// 1. Tạo một iterator cho mỗi SSTable
	iters := make([]Iterator, 0, len(ssts))
	for _, path := range ssts {
		// NewSSTableIterator (từ iterator.go) đọc SST hiệu quả
		it, err := NewSSTableIterator(path)
		if err != nil {
			// Đóng tất cả iterator đã mở nếu có lỗi
			for _, openedIt := range iters {
				openedIt.Close()
			}
			return err
		}
		iters = append(iters, it)
	}

	// 2. Tạo MergingIterator
	// (Nó sẽ tự động xử lý de-dup và tombstones)
	mergedIter := NewMergingIterator(iters)
	defer mergedIter.Close()

	// 3. Chuẩn bị tệp SST mới
	e.mu.Lock() // Bảo vệ e.seq
	e.seq++
	seq := e.seq
	e.mu.Unlock()

	// (Giả sử L1, mặc dù chúng ta chưa có logic cấp độ đầy đủ)
	tempPath := filepath.Join(e.sstDir, "temp-compaction.tmp")
	writer, err := NewSSTWriter(tempPath, 0) // 0 = unknown size
	if err != nil {
		return err
	}

	// 4. Lặp và Ghi (Stream)
	hasEntries := false
	for mergedIter.Next() {
		// MergingIterator đã tự động bỏ qua tombstones [cite: 433-435]
		if err := writer.WriteEntry(mergedIter.Key(), mergedIter.Value()); err != nil {
			writer.Close()
			os.Remove(tempPath)
			return err
		}
		hasEntries = true
	}
	if err := mergedIter.Error(); err != nil {
		writer.Close()
		os.Remove(tempPath)
		return err
	}

	if err := writer.Close(); err != nil {
		os.Remove(tempPath)
		return err
	}

	// Nếu không có entry nào (tất cả đều là rác), chỉ cần xóa file
	if !hasEntries {
		os.Remove(tempPath)
	} else {
		// Đổi tên tệp SST mới
		newPath := filepath.Join(e.sstDir, fmt.Sprintf("sst-L1-%06d.sst", seq))
		if err := os.Rename(tempPath, newPath); err != nil {
			os.Remove(tempPath)
			return err
		}
	}

	e.metrics.compacts.Add(1)

	// 5. Xóa các tệp SST cũ
	for _, p := range ssts {
		_ = os.Remove(p)
	}
	return nil
}
