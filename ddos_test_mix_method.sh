#!/bin/bash

# --- Cấu hình ---
MAX_PARALLEL_JOBS=100
# DOCS_PER_REQUEST=200 # Không dùng cho insertOne
COLLECTIONS=("test1" "test2" "test3" "test4" "test5") 
NUM_COLLECTIONS=${#COLLECTIONS[@]}
BASE_API_URL="http://localhost:6866/api"

# --- Headers ---
H_CONTENT="Content-Type: application/json"
H_ACCEPT="Accept: application/json"
H_UA="User-Agent: ddos-mix-script-v6-mongoId"

# === File log lỗi ===
ERROR_LOG="/tmp/ddos_errors.log"
> $ERROR_LOG
echo "File log lỗi sẽ được ghi tại: $ERROR_LOG"

# === Bộ đếm ===
TOTAL_INSERT_ONE_REQS=0
TOTAL_PUT_REQS=0
TOTAL_DELETE_REQS=0
TOTAL_GET_REQS=0
TOTAL_SEARCH_REQS=0


echo "--- Bắt đầu Stress Test (CRUD FOCUSED + MongoDB-like ID) ---"
echo "Sử dụng ID dạng hex 24 ký tự (timestamp + random)."
echo "Nhấn Ctrl+C để dừng và xem thống kê đối soát."
echo "--------------------------"

# === MỚI: Hàm tạo ID duy nhất kiểu MongoDB ===
# Kết hợp timestamp (hex) và 8 byte ngẫu nhiên (hex) -> 24 ký tự hex
generate_unique_id() {
    local timestamp_hex=$(printf '%x' $(date +%s))
    # Lấy 8 byte ngẫu nhiên, chuyển sang hex, xóa khoảng trắng/xuống dòng
    local random_hex=$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n')
    echo "${timestamp_hex}${random_hex}"
}


# --- Hàm tạo payload (Đã cập nhật để dùng ID mới) ---
generate_single_payload() {
    local unique_id=$(generate_unique_id) # Gọi hàm tạo ID mới
    local ts=$(date +%s)
    
    echo "{\"_id\":\"${unique_id}\",\"name\":\"Load Test Item\",\"ts\":${ts},\"value\":\"$RANDOM\"}"
}

# === Hàm Cleanup và Thống kê (Giữ nguyên logic, chỉ đổi tên biến) ===
cleanup() {
    echo "\nĐang dừng load test... Chờ các job đang chạy hoàn thành."
    wait
    echo "\n--- HOÀN THÀNH LOAD TEST ---"

    # ... (Phần đếm lỗi và tính OK giữ nguyên) ...
    local TOTAL_FAILS=0
    local FAIL_INSERT_ONE=0
    local FAIL_SEARCH=0
    local FAIL_GET=0
    local FAIL_PUT=0
    local FAIL_DELETE=0

    if [ -f "$ERROR_LOG" ]; then
        TOTAL_FAILS=$(wc -l < "$ERROR_LOG")
        FAIL_INSERT_ONE=$(grep -c "TYPE: INSERT_ONE" "$ERROR_LOG")
        FAIL_SEARCH=$(grep -c "TYPE: SEARCH" "$ERROR_LOG")
        FAIL_GET=$(grep -c "TYPE: GET" "$ERROR_LOG")
        FAIL_PUT=$(grep -c "TYPE: PUT" "$ERROR_LOG")
        FAIL_DELETE=$(grep -c "TYPE: DELETE" "$ERROR_LOG")
    fi

    local OK_INSERT_ONE=$((TOTAL_INSERT_ONE_REQS - FAIL_INSERT_ONE))
    local OK_SEARCH=$((TOTAL_SEARCH_REQS - FAIL_SEARCH))
    local OK_GET=$((TOTAL_GET_REQS - FAIL_GET))
    local OK_PUT=$((TOTAL_PUT_REQS - FAIL_PUT))
    local OK_DELETE=$((TOTAL_DELETE_REQS - FAIL_DELETE))
    local TOTAL_DOCS_INSERT_OK=$OK_INSERT_ONE

    # In bảng thống kê chi tiết (Giữ nguyên)
    echo -e "\n--- THỐNG KÊ ĐÃ GỬI (CLIENT-SIDE ATTEMPTS) ---"
    echo "--------------------------------------------------------"
    echo "LOẠI REQUEST | GỬI (ATTEMPTS) | LỖI (FAILS) | THÀNH CÔNG (OK)"
    echo "--------------------------------------------------------"
    printf "insertOne    | %-14d | %-11d | %-13d\n" $TOTAL_INSERT_ONE_REQS $FAIL_INSERT_ONE $OK_INSERT_ONE
    printf "_search      | %-14d | %-11d | %-13d\n" $TOTAL_SEARCH_REQS $FAIL_SEARCH $OK_SEARCH
    printf "GET          | %-14d | %-11d | %-13d\n" $TOTAL_GET_REQS $FAIL_GET $OK_GET
    printf "PUT (upsert) | %-14d | %-11d | %-13d\n" $TOTAL_PUT_REQS $FAIL_PUT $OK_PUT
    printf "DELETE       | %-14d | %-11d | %-13d\n" $TOTAL_DELETE_REQS $FAIL_DELETE $OK_DELETE
    echo "--------------------------------------------------------"
    echo "TỔNG SỐ LỖI: $TOTAL_FAILS"
    echo "(Xem chi tiết lỗi tại $ERROR_LOG)"

    # Thống kê đã LƯU (Server-side) (Giữ nguyên)
    echo -e "\n--- THỐNG KÊ ĐÃ LƯU (SERVER-SIDE) ---"
    echo "Đang truy vấn /_collections để kiểm tra..."
    if ! command -v jq &> /dev/null; then echo "LỖI: Cần cài đặt 'jq'."; exit 1; fi
    RESPONSE_JSON=$(curl -s -f -X GET -H "$H_ACCEPT" "$BASE_API_URL/_collections")
    if [ $? -ne 0 ]; then echo "LỖI: Không thể kết nối tới server."; exit 1; fi
    local TOTAL_DOCS_STORED=$(echo "$RESPONSE_JSON" | jq '[.[] | select(.name | test("test[1-5]")) | .docCount] | add')
    echo "Tổng số document (trong test1-test5): $TOTAL_DOCS_STORED"

    # So sánh (Giữ nguyên)
    echo -e "\n--- ĐỐI SOÁT (TƯƠNG ĐỐI) ---"
    echo "Docs (ước tính) đã GỬI THÀNH CÔNG (từ insertOne): $TOTAL_DOCS_INSERT_OK"
    echo "Docs (ước tính) đã PUT THÀNH CÔNG (tạo mới/cập nhật): $OK_PUT"
    echo "Docs (ước tính) đã DELETE THÀNH CÔNG: $OK_DELETE"
    echo
    echo "SỐ DOCS THỰC TẾ TRÊN SERVER: $TOTAL_DOCS_STORED"
    echo "--------------------------------------------------------"
    echo "(Lý tưởng: TOTAL_DOCS_STORED ~ (TOTAL_DOCS_INSERT_OK + OK_PUT) - OK_DELETE)"
    echo "(Lưu ý: PUT có thể là cập nhật (không tăng count), DELETE có thể target doc không tồn tại)"

    exit 0
}
trap cleanup INT

job_count=0

# Vòng lặp vô hạn
while true; do
    # --- 1. CHUẨN BỊ DỮ LIỆU NGẪU NHIÊN ---
    COL_INDEX=$(($RANDOM % $NUM_COLLECTIONS))
    CURRENT_COLLECTION=${COLLECTIONS[$COL_INDEX]}
    
    # === THAY ĐỔI: Tạo ID kiểu mới cho GET/PUT/DELETE ===
    # Lưu ý: ID này có thể không tồn tại trong DB, dẫn đến 404.
    # Đây là cách đơn giản nhất trong bash.
    CURRENT_ID=$(generate_unique_id) 
    
    CURRENT_TS=$(date +%s)
    OP_CHOICE=$(($RANDOM % 8)) 
    CURL_ARGS=("-s" "-o" "/dev/null" "-H" "$H_CONTENT" "-H" "$H_ACCEPT" "-H" "$H_UA")
    OP_TYPE="UNKNOWN"

    # --- 2. QUYẾT ĐỊNH HÀNH ĐỘNG VÀ TĂNG BỘ ĐẾM ---
    
    case $OP_CHOICE in
        0|1)
            # === HÀNH ĐỘNG: insertOne (WRITE) ===
            OP_TYPE="INSERT_ONE"
            ((TOTAL_INSERT_ONE_REQS++))
            API_URL="${BASE_API_URL}/${CURRENT_COLLECTION}"
            PAYLOAD=$(generate_single_payload) # Đã dùng ID mới bên trong
            CURL_ARGS+=("-X" "POST" "-d" "$PAYLOAD" "$API_URL")
            ;;
        2|3)
            # === HÀNH ĐỘNG: _search (READ) ===
            OP_TYPE="SEARCH"
            ((TOTAL_SEARCH_REQS++))
            API_URL="${BASE_API_URL}/${CURRENT_COLLECTION}/_search"
            PAYLOAD="{\"value\": \"$RANDOM\"}" # Search vẫn dùng value ngẫu nhiên
            CURL_ARGS+=("-X" "POST" "-d" "$PAYLOAD" "$API_URL")
            ;;
        4|5)
            # === HÀNH ĐỘNG: GET (READ) ===
            OP_TYPE="GET"
            ((TOTAL_GET_REQS++))
            # Dùng CURRENT_ID đã tạo ở trên
            API_URL="${BASE_API_URL}/${CURRENT_COLLECTION}/${CURRENT_ID}"
            CURL_ARGS+=("-X" "GET" "$API_URL")
            ;;
        # 6)
        #     # === HÀNH ĐỘNG: PUT (UPSERT - WRITE) ===
        #     OP_TYPE="PUT"
        #     ((TOTAL_PUT_REQS++))
        #     # Dùng CURRENT_ID đã tạo ở trên
        #     API_URL="${BASE_API_URL}/${CURRENT_COLLECTION}/${CURRENT_ID}"
        #     # Payload cần dùng đúng CURRENT_ID
        #     PAYLOAD="{\"_id\":\"${CURRENT_ID}\",\"name\":\"Load Test Item - Update\",\"ts\":${CURRENT_TS},\"value\":\"$RANDOM\"}"
        #     CURL_ARGS+=("-X" "PUT" "-d" "$PAYLOAD" "$API_URL")
        #     ;;
        # 7)
        #     # === HÀNH ĐỘNG: DELETE (WRITE) ===
        #     OP_TYPE="DELETE"
        #     ((TOTAL_DELETE_REQS++))
        #     # Dùng CURRENT_ID đã tạo ở trên
        #     API_URL="${BASE_API_URL}/${CURRENT_COLLECTION}/${CURRENT_ID}"
        #     CURL_ARGS+=("-X" "DELETE" "$API_URL")
        #     ;;
    esac

    # --- 3. THỰC THI & QUẢN LÝ JOB (GHI LOG LỖI) ---
    {
        CURL_STDERR=$(curl "${CURL_ARGS[@]}" 2>&1)
        CURL_EXIT_CODE=$?
        if [ $CURL_EXIT_CODE -ne 0 ]; then
            echo "TYPE: $OP_TYPE | EXIT_CODE: $CURL_EXIT_CODE | ERROR: $CURL_STDERR" >> $ERROR_LOG
        fi
    } &
    
    ((job_count++))

    if [ $job_count -ge $MAX_PARALLEL_JOBS ]; then
        echo -n "." 
        wait 
        job_count=0 
    fi
done