#!/bin/bash

# --- Cấu hình ---
MAX_PARALLEL_JOBS=30
DOCS_PER_REQUEST=200
COLLECTIONS=("test1" "test2" "test3" "test4" "test5")
NUM_COLLECTIONS=${#COLLECTIONS[@]}
BASE_API_URL="http://localhost:6866/api"

# --- Headers ---
H_CONTENT="Content-Type: application/json"
H_ACCEPT="Accept: application/json"
H_UA="User-Agent: ddos-mix-script-v2-stats"

# === MỚI: Bộ đếm (Counters) để thống kê ===
# Chúng ta chỉ quan tâm đến các hành động thay đổi số lượng document
TOTAL_INSERT_MANY_REQS=0
TOTAL_DOCS_REQUESTED_INSERT=0 # Số doc GỬI trong _insertMany
TOTAL_PUT_REQS=0             # PUT có thể là TẠO MỚI hoặc CẬP NHẬT
TOTAL_DELETE_REQS=0          # DELETE sẽ GIẢM
# Các bộ đếm phụ (để xem cho đủ)
TOTAL_GET_REQS=0
TOTAL_SEARCH_REQS=0
TOTAL_COMPACT_REQS=0
TOTAL_COLLECTION_REQS=0

echo "--- Bắt đầu Stress Test (MIXED + STATS) ---"
echo "Spam ngẫu nhiên vào ${NUM_COLLECTIONS} collections."
echo "Số job song song (curl): $MAX_PARALLEL_JOBS"
echo "Số document / 1 _insertMany: $DOCS_PER_REQUEST"
echo "Nhấn Ctrl+C để dừng và xem thống kê."
echo "--------------------------"


# --- Hàm tạo payload (Giữ nguyên) ---
generate_payload() {
    local num_docs=$1
    local base_id=$(date +%s%N)
    local ts=$(date +%s)
    local payload_body=""
    payload_body+="{\"_id\":\"${base_id}_1\",\"name\":\"Load Test Item\",\"ts\":${ts},\"value\":\"$RANDOM\"}"
    for i in $(seq 2 $num_docs); do
        payload_body+=",{\"_id\":\"${base_id}_${i}\",\"name\":\"Load Test Item\",\"ts\":${ts},\"value\":\"$RANDOM\"}"
    done
    echo "[${payload_body}]"
}

# === MỚI: Hàm Cleanup và Thống kê ===
cleanup() {
    echo "\nĐang dừng load test... Chờ các job đang chạy hoàn thành."
    wait
    echo "\n--- HOÀN THÀNH LOAD TEST ---"

    # 1. Thống kê đã GỬI (Client-side)
    echo -e "\n--- THỐNG KÊ ĐÃ GỬI (CLIENT-SIDE) ---"
    echo "Tổng request _insertMany: $TOTAL_INSERT_MANY_REQS"
    echo "Tổng document (trong _insertMany): $TOTAL_DOCS_REQUESTED_INSERT"
    echo "Tổng request PUT (upsert): $TOTAL_PUT_REQS"
    echo "Tổng request DELETE: $TOTAL_DELETE_REQS"
    echo "-----------------------------------"
    echo "Tổng request GET: $TOTAL_GET_REQS"
    echo "Tổng request _search: $TOTAL_SEARCH_REQS"
    echo "Tổng request _compact: $TOTAL_COMPACT_REQS"
    echo "Tổng request _collections: $TOTAL_COLLECTION_REQS"

    # 2. Thống kê đã LƯU (Server-side)
    echo -e "\n--- THỐNG KÊ ĐÃ LƯU (SERVER-SIDE) ---"
    echo "Đang truy vấn /_collections để kiểm tra..."

    # Kiểm tra xem 'jq' có được cài đặt không
    if ! command -v jq &> /dev/null; then
        echo "LỖI: Cần cài đặt 'jq' để xem thống kê."
        echo "Trên macOS: brew install jq"
        echo "Trên Ubuntu: sudo apt-get install jq"
        exit 1
    fi

    # Gọi API và dùng jq để xử lý
    RESPONSE_JSON=$(curl -s -f -X GET -H "$H_ACCEPT" "$BASE_API_URL/_collections")
    
    if [ $? -ne 0 ]; then
        echo "LỖI: Không thể kết nối tới $BASE_API_URL/_collections. Server có thể đã sập."
        exit 1
    fi

    # Tính tổng số document và dung lượng CHỈ CỦA CÁC COLLECTION "test..."
    TOTAL_DOCS_STORED=$(echo "$RESPONSE_JSON" | jq '[.[] | select(.name | test("test[1-5]")) | .docCount] | add')
    TOTAL_SIZE_BYTES=$(echo "$RESPONSE_JSON" | jq '[.[] | select(.name | test("test[1-5]")) | .byteSize] | add')

    # Chuyển đổi dung lượng (nếu có thể)
    if command -v numfmt &> /dev/null; then
        TOTAL_SIZE_READABLE=$(numfmt --to=iec-i --suffix=B --format="%.2f" $TOTAL_SIZE_BYTES)
    else
        TOTAL_SIZE_READABLE="${TOTAL_SIZE_BYTES} Bytes"
    fi

    echo "Tổng số document (trong test1-5): $TOTAL_DOCS_STORED"
    echo "Tổng dung lượng (trong test1-5): $TOTAL_SIZE_READABLE"
    
    # 3. So sánh
    echo -e "\n--- SO SÁNH (CHỈ MANG TÍNH TƯƠNG ĐỐI) ---"
    echo "Docs đã gửi (chỉ từ _insertMany): $TOTAL_DOCS_REQUESTED_INSERT"
    echo "Docs hiện có (sau khi trừ DELETE): $TOTAL_DOCS_STORED"
    
    echo "(Lưu ý: Số 'Đã gửi' chưa tính PUT (có thể tạo mới). 'Docs hiện có' là tổng cuối cùng sau khi đã trừ DELETE.)"

    exit 0
}
trap cleanup INT

job_count=0

# Vòng lặp vô hạn
while true; do
    # --- 1. CHUẨN BỊ DỮ LIỆU NGẪU NHIÊN ---
    COL_INDEX=$(($RANDOM % $NUM_COLLECTIONS))
    CURRENT_COLLECTION=${COLLECTIONS[$COL_INDEX]}
    CURRENT_ID="$(date +%s)N_$(($RANDOM % 100))"
    CURRENT_TS=$(date +%s)
    OP_CHOICE=$(($RANDOM % 10))
    CURL_ARGS=("-s" "-o" "/dev/null" "-H" "$H_CONTENT" "-H" "$H_ACCEPT" "-H" "$H_UA")

    # --- 2. QUYẾT ĐỊNH HÀNH ĐỘNG VÀ TĂNG BỘ ĐẾM ---
    
    case $OP_CHOICE in
        0|1)
            # === HÀNH ĐỘNG: _insertMany (WRITE NẶNG) ===
            ((TOTAL_INSERT_MANY_REQS++))
            ((TOTAL_DOCS_REQUESTED_INSERT += DOCS_PER_REQUEST))
            
            API_URL="${BASE_API_URL}/${CURRENT_COLLECTION}/_insertMany"
            PAYLOAD=$(generate_payload $DOCS_PER_REQUEST)
            CURL_ARGS+=("-X" "POST" "-d" "$PAYLOAD" "$API_URL")
            ;;
        2|3)
            # === HÀNH ĐỘNG: _search (READ) ===
            ((TOTAL_SEARCH_REQS++))
            API_URL="${BASE_API_URL}/${CURRENT_COLLECTION}/_search"
            PAYLOAD="{\"value\": \"$RANDOM\"}"
            CURL_ARGS+=("-X" "POST" "-d" "$PAYLOAD" "$API_URL")
            ;;
        4|5)
            # === HÀNH ĐỘNG: GET (READ) ===
            ((TOTAL_GET_REQS++))
            API_URL="${BASE_API_URL}/${CURRENT_COLLECTION}/${CURRENT_ID}"
            CURL_ARGS+=("-X" "GET" "$API_URL")
            ;;
        6)
            # === HÀNH ĐỘNG: PUT (UPSERT - WRITE) ===
            ((TOTAL_PUT_REQS++))
            API_URL="${BASE_API_URL}/${CURRENT_COLLECTION}/${CURRENT_ID}"
            PAYLOAD="{\"_id\":\"${CURRENT_ID}\",\"name\":\"Load Test Item - Update\",\"ts\":${CURRENT_TS},\"value\":\"$RANDOM\"}"
            CURL_ARGS+=("-X" "PUT" "-d" "$PAYLOAD" "$API_URL")
            ;;
        7)
            # === HÀNH ĐỘNG: DELETE (WRITE) ===
            ((TOTAL_DELETE_REQS++))
            API_URL="${BASE_API_URL}/${CURRENT_COLLECTION}/${CURRENT_ID}"
            CURL_ARGS+=("-X" "DELETE" "$API_URL")
            ;;
        8)
            # === HÀNH ĐỘNG: _collections (META-READ) ===
            ((TOTAL_COLLECTION_REQS++))
            API_URL="${BASE_API_URL}/_collections"
            CURL_ARGS+=("-X" "GET" "$API_URL")
            ;;
        *)
            # === HÀNH ĐỘNG: _compact (MAINTENANCE) ===
            ((TOTAL_COMPACT_REQS++))
            API_URL="${BASE_API_URL}/_compact"
            CURL_ARGS+=("-X" "POST" "$API_URL")
            ;;
    esac

    # --- 3. THỰC THI & QUẢN LÝ JOB ---
    
    curl "${CURL_ARGS[@]}" &
    ((job_count++))

    if [ $job_count -ge $MAX_PARALLEL_JOBS ]; then
        echo -n "." # In ra dấu chấm để biết script vẫn đang chạy
        wait 
        job_count=0 
    fi
done