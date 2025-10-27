#!/bin/bash

# --- Cấu hình ---
MAX_PARALLEL_JOBS=100
# === MỚI: Số lượng document trong MỘT request _insertMany ===
# Tăng số này để gửi nhiều data hơn mỗi lần curl
DOCS_PER_REQUEST=200 

COLLECTIONS=("test1" "test2" "test3" "test4" "test5")
NUM_COLLECTIONS=${#COLLECTIONS[@]}
BASE_API_URL="http://localhost:6866/api"

echo "--- Bắt đầu Stress Test (với Batch Insert) ---"
echo "Spam ngẫu nhiên vào ${NUM_COLLECTIONS} collections."
echo "Số job song song (curl): $MAX_PARALLEL_JOBS"
echo "Số document / request: $DOCS_PER_REQUEST"
echo "TỔNG SỐ DOCS MỖI BATCH: $(($MAX_PARALLEL_JOBS * $DOCS_PER_REQUEST))" # Cập nhật thông báo
echo "Nhấn Ctrl+C để dừng."
echo "--------------------------"

# === MỚI: Hàm tạo payload "đủ lớn" ===
# Tạo ra một chuỗi JSON array chứa $1 document
# (Tối ưu hóa để chạy nhanh trong shell)
generate_payload() {
    local num_docs=$1
    # Dùng 1 timestamp nano cơ sở cho batch này để đảm bảo ID khác nhau
    local base_id=$(date +%s%N) 
    local ts=$(date +%s)       # Dùng 1 timestamp giây chung cho batch
    
    local payload_body=""
    
    # Tạo document đầu tiên (không có dấu phẩy ở trước)
    # Thêm số thứ tự vào ID để chắc chắn không trùng trong batch
    payload_body+="{\"_id\":\"${base_id}_1\",\"name\":\"Load Test Item\",\"ts\":${ts},\"value\":\"$RANDOM\"}" 
    
    # Tạo các document còn lại (có dấu phẩy ở trước)
    for i in $(seq 2 $num_docs); do
        # Thêm $RANDOM vào value để khác nhau
        payload_body+=",{\"_id\":\"${base_id}_${i}\",\"name\":\"Load Test Item\",\"ts\":${ts},\"value\":\"$RANDOM\"}" 
    done
    
    # Trả về mảng JSON hoàn chỉnh
    echo "[${payload_body}]"
}


# Hàm dọn dẹp khi nhấn Ctrl+C
cleanup() {
    echo "\nĐang dừng load test... Chờ các job đang chạy hoàn thành."
    wait
    echo "Tất cả đã xong. Thoát."
    exit 0
}
trap cleanup INT

job_count=0

# Vòng lặp vô hạn
while true; do
    # Chọn collection ngẫu nhiên
    COL_INDEX=$(($RANDOM % $NUM_COLLECTIONS))
    CURRENT_COLLECTION=${COLLECTIONS[$COL_INDEX]}
    CURRENT_API_ENDPOINT="${BASE_API_URL}/${CURRENT_COLLECTION}/_insertMany"

    # === THAY ĐỔI: Tạo payload "lớn" ===
    # Gọi hàm để tạo ra payload chứa $DOCS_PER_REQUEST document
    PAYLOAD=$(generate_payload $DOCS_PER_REQUEST) 

    # Gửi request trong background (logic giữ nguyên)
    curl -s -o /dev/null -X POST -H "Content-Type: application/json" -d "$PAYLOAD" "$CURRENT_API_ENDPOINT" &

    ((job_count++))

    # Kiểm soát số lượng job song song (logic giữ nguyên)
    if [ $job_count -ge $MAX_PARALLEL_JOBS ]; then
        # Cập nhật thông báo
        echo -n " (Đã gửi $job_count jobs x $DOCS_PER_REQUEST docs. Đang chờ hoàn tất...)" 
        wait
        job_count=0 
        echo " Xong. Gửi batch tiếp theo."
    fi
done