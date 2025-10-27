#!/bin/bash

# --- Cấu hình ---
# Lấy số lượng collection cần tạo từ đối số (mặc định 3)
COLLECTION_COUNT=${1:-3}
# Số record tối thiểu/tối đa cho mỗi collection
MIN_RECORDS=20
MAX_RECORDS=200 
# Số field tối thiểu/tối đa cho mỗi collection (ngoài _id)
MIN_FIELDS=8
MAX_FIELDS=16
# --- Kích thước mỗi batch insert ---
BATCH_SIZE=50

# Địa chỉ API
API_ENDPOINT="http://localhost:6866/api"

echo "Generating data for ${COLLECTION_COUNT} random collections via API (using batches of ${BATCH_SIZE})..."
echo "--- Sending requests in the background (will run faster) ---" # <-- Thông báo mới
echo "---"

# --- Dữ liệu mẫu (Giữ nguyên) ---
FIRST_NAMES=("An" "Bao" "Chi" "Dung" "Giang" "Hieu" "Khanh" "Linh" "Minh" "Nam" "Phong" "Quynh" "Son" "Thao" "Tuan" "Viet" "Anh" "Huong" "My" "Phuc" "John" "Jane" "Peter" "Mary")
LAST_NAMES=("Nguyen" "Tran" "Le" "Pham" "Hoang" "Huynh" "Phan" "Vo" "Dang" "Bui" "Do" "Ngo" "Duong" "Ly" "Vu" "Smith" "Jones" "Williams")
CITIES=("Ha Noi" "Ho Chi Minh" "Da Nang" "Hai Phong" "Can Tho" "Bien Hoa" "Nha Trang" "Vung Tau" "Quy Nhon" "Hue" "London" "Paris" "Tokyo" "New York")
DOMAINS=("gmail.com" "yahoo.com" "outlook.com" "proton.me" "company.com" "domain.net" "mail.org")
STREETS=("Le Loi" "Tran Hung Dao" "Nguyen Hue" "Ly Thuong Kiet" "Quang Trung" "Hai Ba Trung" "Vo Nguyen Giap" "Pham Van Dong" "Baker St" "Elm St")
WORDS=("lorem" "ipsum" "dolor" "sit" "amet" "consectetur" "adipiscing" "elit" "sed" "do" "eiusmod" "tempor" "incididunt" "ut" "labore" "et" "dolore" "magna" "aliqua")
STATUS_OPTIONS=("pending" "processing" "shipped" "delivered" "cancelled" "returned")
CATEGORIES=("electronics" "apparel" "books" "home" "grocery" "toys" "sports" "tools")
TAGS=("new" "sale" "featured" "popular" "limited" "eco-friendly")
# ---------------------------------------------------

# Lấy số lượng phần tử trong các mảng (Giữ nguyên)
num_first_names=${#FIRST_NAMES[@]}
num_last_names=${#LAST_NAMES[@]}
num_cities=${#CITIES[@]}
num_domains=${#DOMAINS[@]}
num_streets=${#STREETS[@]}
num_words=${#WORDS[@]}
num_status=${#STATUS_OPTIONS[@]}
num_categories=${#CATEGORIES[@]}
num_tags=${#TAGS[@]}

# --- Định nghĩa các loại field có thể có (Giữ nguyên) ---
POSSIBLE_FIELDS=(
  "name:name" "title:string" "description:words" "email:email" "phone:phone"
  "city:city" "address:address" "country:string" "zip_code:number:10000:99999"
  "price:number:10:5000" "quantity:number:1:100" "age:number:18:99" "rating:number:1:5"
  "discount:number:0:50" "is_active:boolean" "is_verified:boolean" "on_sale:boolean"
  "created_at:date" "updated_at:date" "last_login:date" "order_date:date"
  "status:status" "category:category" "tags:tags" "notes:words"
)
num_possible_fields=${#POSSIBLE_FIELDS[@]}

# --- Danh sách tên collection có thể (Giữ nguyên) ---
POSSIBLE_COLLECTIONS=("orders" "users" "products" "inventory" "logs" "reviews" "sessions" "carts")
num_possible_collections=${#POSSIBLE_COLLECTIONS[@]}

# --- Hàm tạo dữ liệu ngẫu nhiên cho từng type (Giữ nguyên) ---
generate_fake_data() {
  local field_info=$1
  local field_name=$(echo $field_info | cut -d: -f1)
  local field_type=$(echo $field_info | cut -d: -f2)

  case $field_type in
    string) echo "\"${WORDS[$(($RANDOM % num_words))]}\"";;
    name) local fn="${FIRST_NAMES[$(($RANDOM % num_first_names))]}"; local ln="${LAST_NAMES[$(($RANDOM % num_last_names))]}"; echo "\"${fn} ${ln}\"";;
    email) local fn="${FIRST_NAMES[$(($RANDOM % num_first_names))]}"; local ln="${LAST_NAMES[$(($RANDOM % num_last_names))]}"; local en=$(echo "${fn}.${ln}$(($RANDOM % 1000))" | tr '[:upper:]' '[:lower:]' | tr -d ' '); local d="${DOMAINS[$(($RANDOM % num_domains))]}"; echo "\"${en}@${d}\"";;
    phone) echo "\"09$(($RANDOM % 10))$(($RANDOM % 10))-$(($RANDOM % 900 + 100))-$(($RANDOM % 9000 + 1000))\"";;
    city) echo "\"${CITIES[$(($RANDOM % num_cities))]}\"";;
    address) local sn="${STREETS[$(($RANDOM % num_streets))]}"; local num=$(($RANDOM % 1000 + 1)); echo "\"${num} ${sn}\"";;
    number) local min=$(echo $field_info | cut -d: -f3); local max=$(echo $field_info | cut -d: -f4); local range=$(($max - $min + 1)); echo "$(($RANDOM % $range + $min))";;
    boolean) if [ $(($RANDOM % 2)) -eq 0 ]; then echo "false"; else echo "true"; fi;;
    date)
      local days_ago=$(($RANDOM % 730))
      # Sử dụng date phù hợp cho cả Linux (GNU) và macOS (BSD)
      if date --version >/dev/null 2>&1; then echo "\"$(date -d "-${days_ago} days" --iso-8601=seconds)\""; else echo "\"$(date -v-${days_ago}d -Iseconds)\""; fi;;
    status) echo "\"${STATUS_OPTIONS[$(($RANDOM % num_status))]}\"";;
    category) echo "\"${CATEGORIES[$(($RANDOM % num_categories))]}\"";;
    tags) local num_c=$(($RANDOM % 3 + 1)); local cs="["; local tt=("${TAGS[@]}"); local nt=${#tt[@]}; for (( k=$nt-1 ; k>0 ; k-- )); do local j=$((RANDOM % (k + 1))); local tmp=${tt[k]}; tt[k]=${tt[j]}; tt[j]=$tmp; done; for (( k=0; k<$num_c; k++ )); do cs+="\"${tt[k]}\""; if [ $k -lt $(($num_c - 1)) ]; then cs+=","; fi; done; echo "$cs]";;
    words) local num_w=$(($RANDOM % 8 + 3)); local s=""; for (( k=0; k<$num_w; k++ )); do s+="${WORDS[$(($RANDOM % num_words))]} "; done; echo "\"${s% }\"";;
    *) echo "\"unknown_type_${field_type}\"";;
  esac
}

# --- Hàm tạo ID giống ObjectID (Giữ nguyên) ---
generate_oid() {
  local timestamp=$(printf '%x' $(date +%s))
  local random_hex=""
  if [ -c /dev/urandom ]; then random_hex=$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n'); else for _ in {1..4}; do random_hex+=$(printf '%04x' $RANDOM); done; fi
  echo "${timestamp}${random_hex}"
}


# --- Vòng lặp chính để tạo collections ---
created_collections=()
for (( c=1; c<=$COLLECTION_COUNT; c++ )); do

  # Chọn tên collection ngẫu nhiên (Giữ nguyên)
  collection_name=""
  while true; do collection_name="${POSSIBLE_COLLECTIONS[$(($RANDOM % num_possible_collections))]}"; is_duplicate=false; for existing_col in "${created_collections[@]}"; do if [[ "$existing_col" == "$collection_name" ]]; then is_duplicate=true; break; fi; done; if ! $is_duplicate; then created_collections+=("$collection_name"); break; fi; if [ ${#created_collections[@]} -eq ${num_possible_collections} ]; then echo "All possible collections generated."; break 2; fi; done

  # Quyết định số record và số field (Giữ nguyên)
  record_count=$(($RANDOM % ($MAX_RECORDS - $MIN_RECORDS + 1) + $MIN_RECORDS))
  field_count=$(($RANDOM % ($MAX_FIELDS - $MIN_FIELDS + 1) + $MIN_FIELDS))

  echo "Generating and sending ${record_count} records for collection '${collection_name}'..."

  # Chọn ngẫu nhiên các field (Giữ nguyên)
  selected_field_indices=()
  selected_fields_info=()
  while [ ${#selected_field_indices[@]} -lt $field_count ]; do random_index=$(($RANDOM % $num_possible_fields)); is_duplicate=false; for index in "${selected_field_indices[@]}"; do if [[ $index -eq $random_index ]]; then is_duplicate=true; break; fi; done; if ! $is_duplicate; then selected_field_indices+=($random_index); selected_fields_info+=("${POSSIBLE_FIELDS[$random_index]}"); fi; done

  # --- VÒNG LẶP Gửi theo Batch (Đã sửa đổi) ---
  
  num_batches=$(( ($record_count + $BATCH_SIZE - 1) / $BATCH_SIZE ))
  current_record_index=0

  echo "Total records: ${record_count}. Total batches: ${num_batches}."

  for (( b=0; b<$num_batches; b++ )); do
    
    remaining_records=$(( $record_count - $current_record_index ))
    if [ $remaining_records -gt $BATCH_SIZE ]; then
      current_batch_size=$BATCH_SIZE
    else
      current_batch_size=$remaining_records
    fi

    if [ $current_batch_size -le 0 ]; then continue; fi

    batch_docs=()
    first_oid_in_batch="" # Vẫn giữ để log

    # 1. TẠO DATA: (Giữ nguyên)
    # echo -n "Generating ${current_batch_size} documents for batch $(($b + 1))... " >&2 # Không cần log này nữa
    for (( i=0; i<$current_batch_size; i++ )); do
      oid=$(generate_oid)
      if [ $i -eq 0 ]; then first_oid_in_batch=$oid; fi
      
      json_doc="{\"_id\":\"${oid}\""
      for field_info in "${selected_fields_info[@]}"; do
        field_name=$(echo $field_info | cut -d: -f1)
        if [[ "$field_name" == "_id" ]]; then continue; fi
        fake_data=$(generate_fake_data "$field_info")
        json_doc+=",\"${field_name}\":${fake_data}"
      done
      json_doc+="}"
      batch_docs+=("$json_doc")
    done
    # echo "Done." >&2 # Không cần log này nữa

    # 2. GỬI API: Gửi batch vừa tạo và CHẠY TRONG BACKGROUND
    batch_payload="[$(IFS=,; echo "${batch_docs[*]}")]"

    batch_num=$(( $b + 1 ))
    record_start=$(( $current_record_index + 1 ))
    record_end=$(( $current_record_index + $current_batch_size ))
    current_record_index=$record_end

    # === THAY ĐỔI: Chạy curl trong background (&) và loại bỏ timing/response capture ===
    echo "Launching batch ${batch_num}/${num_batches} (records ${record_start}-${record_end}) for '${collection_name}' (First ID: ${first_oid_in_batch})..."
    
    # -s: silent, -o /dev/null: discard output, &: run in background
    curl -s -o /dev/null -X POST -H "Content-Type: application/json" -d "$batch_payload" "${API_ENDPOINT}/${collection_name}/_insertMany" &

  done

  echo "--- Finished launching batches for collection ${collection_name} ---"

done

# === THAY ĐỔI: Chờ tất cả các job background hoàn thành ===
echo "All batches launched. Waiting for background jobs to complete..."
wait
echo "All background jobs finished."

echo "All data generation complete."