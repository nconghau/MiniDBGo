#!/bin/bash

# --- Cấu hình ---
BASE_URL="http://localhost:6866/api"
# Số lượng request song song cho mỗi loại API (Tổng cộng = CONCURRENCY * 3)
CONCURRENCY=50

# --- SỬA ĐỔI: Mảng các collections để test ---
POSSIBLE_COLLECTIONS=("orders" "users" "products" "inventory" "logs" "reviews" "sessions" "carts")
NUM_COLLECTIONS=${#POSSIBLE_COLLECTIONS[@]}
# --- KẾT THÚC SỬA ---


# --- Hàm trợ giúp ---

# Tạo ID 24 ký tự hex (12 bytes) DỰA TRÊN THỜI GIAN (ms) + NGẪU NHIÊN
# Gần giống với MongoDB ObjectId (kết hợp time + random)
generate_unique_id() {
  # --- SỬA ĐỔI: Kiểm tra python3 thay vì date ---
  # Kiểm tra xem 'openssl' và 'python3' có tồn tại không
  if ! command -v openssl &> /dev/null || ! command -v python3 &> /dev/null
  then
    echo "Lỗi: 'openssl' hoặc 'python3' không được cài đặt." >&2
    exit 1
  fi
  # --- KẾT THÚC SỬA ---

  # --- SỬA ĐỔI: Sử dụng python3 để lấy timestamp (tương thích đa nền tảng) ---
  # 1. Lấy 6-byte timestamp (tính bằng mili-giây)
  local ms_timestamp
  ms_timestamp=$(python3 -c 'import time; print(int(time.time() * 1000))')
  
  # '%012x' -> Chuyển sang hex, đệm trái bằng '0' cho đủ 12 ký tự (6 bytes)
  local timestamp_hex
  timestamp_hex=$(printf '%012x' "$ms_timestamp")
  # --- KẾT THÚC SỬA ---

  # 2. Lấy 6-byte ngẫu nhiên (12 ký tự hex)
  local random_hex
  random_hex=$(openssl rand -hex 6)

  # 3. Kết hợp lại (Tổng 12 bytes / 24 ký tự hex)
  echo "${timestamp_hex}${random_hex}"
}

# --- SỬA ĐỔI: Hàm mới: Lấy một collection ngẫu nhiên ---
get_random_collection() {
  local index=$((RANDOM % NUM_COLLECTIONS))
  echo "${POSSIBLE_COLLECTIONS[$index]}"
}
# --- KẾT THÚC SỬA ---


# --- Các hàm Test API ---

# 1. Test Health Check
test_health_check() {
  curl -s -o /dev/null "$BASE_URL/health"
}

# 2. Test InsertOne
test_insert_one() {
  local ID=$(generate_unique_id)
  local COLLECTION=$(get_random_collection) # SỬA ĐỔI
  
  # Sử dụng một trong các bản ghi mẫu của bạn, thay thế ID
  local PAYLOAD
  PAYLOAD=$(cat <<EOF
{
  "_id": "$ID",
  "age": 18,
  "category": "electronics",
  "city": "Paris",
  "country": "eiusmod",
  "is_active": true,
  "is_verified": true,
  "last_login": "2025-06-28T10:25:46+07:00",
  "price": 3770,
  "rating": 1,
  "tags": ["eco-friendly"],
  "title": "test_insert_one"
}
EOF
)

  curl -s -o /dev/null -X POST \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$BASE_URL/$COLLECTION" # SỬA ĐỔI
}

# 3. Test InsertMany (2 bản ghi)
test_insert_many() {
  local ID1=$(generate_unique_id)
  local ID2=$(generate_unique_id)
  local COLLECTION=$(get_random_collection) # SỬA ĐỔI
  
  # Sử dụng 2 bản ghi mẫu
  local PAYLOAD
  PAYLOAD=$(cat <<EOF
[
  {
    "_id": "$ID1",
    "age": 40,
    "category": "electronics",
    "city": "Ho Chi Minh",
    "country": "eiusmod",
    "is_active": true,
    "is_verified": false,
    "last_login": "2024-05-02T10:25:43+07:00",
    "price": 780,
    "rating": 3,
    "tags": ["new", "popular", "sale"],
    "title": "test_insert_many_1"
  },
  {
    "_id": "$ID2",
    "age": 92,
    "category": "electronics",
    "city": "Hai Phong",
    "country": "magna",
    "is_active": true,
    "is_verified": true,
    "last_login": "2025-02-06T10:10:13+07:00",
    "price": 225,
    "rating": 5,
    "tags": ["popular", "eco-friendly", "new"],
    "title": "test_insert_many_2"
  }
]
EOF
)

  curl -s -o /dev/null -X POST \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$BASE_URL/$COLLECTION/_insertMany" # SỬA ĐỔI
}


# --- Vòng lặp chính ---
echo "Bắt đầu Stress Test trên $BASE_URL với $CONCURRENCY workers (trên $NUM_COLLECTIONS collections)..."
echo "Nhấn [Ctrl+C] để dừng."

# Vòng lặp vô hạn
while true; do
  
  # Chạy $CONCURRENCY jobs cho mỗi hàm trong nền
  for ((i=1; i<=CONCURRENCY; i++)); do
    test_health_check &
    test_insert_one &
    test_insert_many &
  done
  
  # Chờ tất cả các tiến trình nền của đợt này hoàn thành
  wait
  
  # Lặp lại ngay lập tức
done

