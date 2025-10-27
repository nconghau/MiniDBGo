#!/bin/bash

# --- Cấu hình ---
# Tăng/giảm số lượng request song song
# Bắt đầu ở mức 10-20, sau đó tăng dần để xem server chịu được bao nhiêu.
MAX_PARALLEL_JOBS=20

# --- MỚI: Danh sách collections để spam ---
# Thêm hoặc bớt collection tùy ý
COLLECTIONS=("test1" "test2" "test3" "test4" "test5")

# Lấy số lượng collection từ mảng
NUM_COLLECTIONS=${#COLLECTIONS[@]}

# Endpoint API cơ sở
BASE_API_URL="http://localhost:6866/api"

# Payload mẫu (sẽ thay đổi _id và ts mỗi lần)
# %ID% và %TS% sẽ được thay thế
PAYLOAD_TEMPLATE='[{"_id":"%ID%","name":"Load Test Item","ts":%TS%,"value":'$RANDOM'}]'

echo "--- Bắt đầu Stress Test ---"
echo "Spam ngẫu nhiên vào ${NUM_COLLECTIONS} collections (ví dụ: ${COLLECTIONS[0]}, ${COLLECTIONS[1]}...)"
echo "Số job song song: $MAX_PARALLEL_JOBS"
echo "Nhấn Ctrl+C để dừng."
echo "--------------------------"

# Hàm dọn dẹp khi nhấn Ctrl+C
cleanup() {
    echo "\nĐang dừng load test... Chờ các job đang chạy hoàn thành."
    # Chờ tất cả các job con kết thúc
    wait
    echo "Tất cả đã xong. Thoát."
    exit 0
}

# Bắt tín hiệu INT (Ctrl+C) và gọi hàm cleanup
trap cleanup INT

job_count=0

# Vòng lặp vô hạn
while true; do
    # --- MỚI: Chọn một collection ngẫu nhiên cho request này ---
    COL_INDEX=$(($RANDOM % $NUM_COLLECTIONS))
    CURRENT_COLLECTION=${COLLECTIONS[$COL_INDEX]}
    CURRENT_API_ENDPOINT="${BASE_API_URL}/${CURRENT_COLLECTION}/_insertMany"

    # Tạo _id và timestamp duy nhất (dùng nanoseconds)
    ID=$(date +%s%N)

    # Tạo payload
    # 1. Thay thế %ID% bằng ID
    PAYLOAD_STEP1=${PAYLOAD_TEMPLATE/\%ID\%/$ID}
    # 2. Thay thế %TS% bằng timestamp (giây)
    PAYLOAD=${PAYLOAD_STEP1/\%TS\%/$(date +%s)}

    # Gửi request trong background (chế độ nền)
    # -s (silent): Im lặng, không có thanh tiến trình
    # -o /dev/null: Vứt bỏ output (không lưu/in ra màn hình)
    #               Việc này cực kỳ quan trọng để giữ cho script chạy nhanh
    curl -s -o /dev/null -X POST -H "Content-Type: application/json" -d "$PAYLOAD" "$CURRENT_API_ENDPOINT" &

    ((job_count++))

    # Kiểm soát số lượng job song song
    # Nếu số job đang chạy >= giới hạn
    if [ $job_count -ge $MAX_PARALLEL_JOBS ]; then
        # --- SỬA LỖI ---
        # Chờ TẤT CẢ các job trong batch này hoàn thành
        # 'wait' không có -n sẽ tương thích với các shell cũ
        echo -n " (Đã gửi $job_count jobs. Đang chờ hoàn tất...)"
        wait
        job_count=0 # Reset bộ đếm
        echo " Xong. Gửi batch tiếp theo."
    fi
done
