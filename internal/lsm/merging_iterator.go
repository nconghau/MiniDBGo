package lsm

import (
	"container/heap"
)

// mergingIteratorItem là một wrapper cho container/heap
// Nó giữ một iterator và giá trị (key/value) hiện tại của nó
type mergingIteratorItem struct {
	iter  Iterator
	key   string
	value *Item
}

// mergingIteratorHeap là một min-heap của các iterator
// (ưu tiên theo `key`)
type mergingIteratorHeap []mergingIteratorItem

func (h mergingIteratorHeap) Len() int { return len(h) }

func (h mergingIteratorHeap) Less(i, j int) bool {
	// Chỉ cần so sánh key
	return h[i].key < h[j].key
	// Nếu key bằng nhau, thứ tự không quan trọng
	// vì logic Next() sẽ xử lý de-dup
}

func (h mergingIteratorHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *mergingIteratorHeap) Push(x interface{}) {
	*h = append(*h, x.(mergingIteratorItem))
}

func (h *mergingIteratorHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// MergingIterator hợp nhất nhiều iterator
type MergingIterator struct {
	h     mergingIteratorHeap
	iters []Iterator // Giữ tham chiếu đầy đủ để Close()

	key   string
	value *Item
	err   error
}

// NewMergingIterator tạo một iterator hợp nhất từ một danh sách các iterator con.
// Danh sách `iters` phải được sắp xếp theo *thứ tự ưu tiên*
// (ví dụ: MemTable, Immutable, rồi mới đến SSTables)
// Mặc dù heap sắp xếp theo key, thứ tự ưu tiên quan trọng khi
// de-dup (nhưng logic de-dup của chúng ta sẽ xử lý cả hai)
//
// Sửa lại: Logic của chúng ta xử lý de-dup và tombstone
// dựa trên giả định rằng iterator mới nhất (MemTable)
// sẽ được xử lý trước NẾU key giống hệt nhau.
//
// Sửa lại (Lần 3): Min-heap chỉ sắp xếp theo key.
// Logic de-dup bên dưới mới là thứ xử lý
// các key bị trùng.
func NewMergingIterator(iters []Iterator) Iterator {
	mi := &MergingIterator{
		h:     make(mergingIteratorHeap, 0, len(iters)),
		iters: iters, // Lưu lại để Close()
	}

	// Khởi tạo heap: Đẩy item đầu tiên của mỗi iterator vào
	for _, iter := range iters {
		if iter.Next() {
			heap.Push(&mi.h, mergingIteratorItem{
				iter:  iter,
				key:   iter.Key(),
				value: iter.Value(),
			})
		}
		if iter.Error() != nil {
			mi.err = iter.Error()
			break
		}
	}

	// Nếu có lỗi trong khi khởi tạo, trả về một iterator lỗi
	if mi.err != nil {
		// Đóng tất cả những gì đã mở
		for _, iter := range iters {
			iter.Close()
		}
		// Trả về iterator rỗng nhưng có lỗi
		return &MergingIterator{err: mi.err}
	}

	return mi
}

// Next là phần logic phức tạp nhất
func (it *MergingIterator) Next() bool {
	if it.err != nil {
		return false
	}

	// Vòng lặp này xử lý các key trùng lặp và tombstone
	for {
		if it.h.Len() == 0 {
			return false // Hết dữ liệu
		}

		// 1. Lấy iterator có key nhỏ nhất (từ đỉnh heap)
		item := heap.Pop(&it.h).(mergingIteratorItem)
		currentKey := item.key
		currentValue := item.value

		// 2. De-duplication (Loại bỏ trùng lặp)
		// Lấy tất cả các iterator khác có *cùng key* ra khỏi heap
		for it.h.Len() > 0 && it.h[0].key == currentKey {
			dupItem := heap.Pop(&it.h).(mergingIteratorItem)
			// Di chuyển con trỏ của iterator bị trùng lặp này
			if dupItem.iter.Next() {
				heap.Push(&it.h, mergingIteratorItem{
					iter:  dupItem.iter,
					key:   dupItem.iter.Key(),
					value: dupItem.iter.Value(),
				})
			} else if dupItem.iter.Error() != nil {
				it.err = dupItem.iter.Error()
				return false
			}
		}

		// 3. Di chuyển con trỏ của iterator chính (item)
		if item.iter.Next() {
			heap.Push(&it.h, mergingIteratorItem{
				iter:  item.iter,
				key:   item.iter.Key(),
				value: item.iter.Value(),
			})
		} else if item.iter.Error() != nil {
			it.err = item.iter.Error()
			return false
		}

		// 4. Xử lý Tombstone
		// Nếu key này (mới nhất) là tombstone,
		// chúng ta bỏ qua nó và lặp lại (để tìm key tiếp theo)
		if currentValue.Tombstone {
			continue
		}

		// 5. Tìm thấy một key hợp lệ!
		it.key = currentKey
		it.value = currentValue
		return true
	}
}

func (it *MergingIterator) Key() string {
	return it.key
}

func (it *MergingIterator) Value() *Item {
	return it.value
}

func (it *MergingIterator) Error() error {
	return it.err
}

func (it *MergingIterator) Close() error {
	var firstErr error
	for _, iter := range it.iters {
		if err := iter.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	// Dọn dẹp
	it.h = nil
	it.iters = nil
	return firstErr
}
