#!/bin/bash

# Lấy số lượng collection cần tạo từ đối số (mặc định 3)
COLLECTION_COUNT=${1:-3}
# Số record tối thiểu/tối đa cho mỗi collection
MIN_RECORDS=50
MAX_RECORDS=200
# Số field tối thiểu/tối đa cho mỗi collection (ngoài _id)
MIN_FIELDS=5
MAX_FIELDS=15

# --- CẬP NHẬT: Địa chỉ API ---
API_ENDPOINT="http://localhost:6866/api"

echo "Generating data for ${COLLECTION_COUNT} random collections via API..."
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
  "name:name"
  "title:string"
  "description:words"
  "email:email"
  "phone:phone"
  "city:city"
  "address:address"
  "country:string" # Sẽ fix thành Vietnam hoặc quốc gia khác
  "zip_code:number:10000:99999"
  "price:number:10:5000"
  "quantity:number:1:100"
  "age:number:18:99"
  "rating:number:1:5"
  "discount:number:0:50"
  "is_active:boolean"
  "is_verified:boolean"
  "on_sale:boolean"
  "created_at:date"
  "updated_at:date"
  "last_login:date"
  "order_date:date"
  "status:status"
  "category:category"
  "tags:tags"
  "notes:words"
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
    string)
      echo "\"${WORDS[$(($RANDOM % num_words))]}\"" ;;
    name)
      local first_name="${FIRST_NAMES[$(($RANDOM % num_first_names))]}"
      local last_name="${LAST_NAMES[$(($RANDOM % num_last_names))]}"
      echo "\"${first_name} ${last_name}\"" ;;
    email)
      local first_name="${FIRST_NAMES[$(($RANDOM % num_first_names))]}"
      local last_name="${LAST_NAMES[$(($RANDOM % num_last_names))]}"
      local email_name=$(echo "${first_name}.${last_name}$(($RANDOM % 1000))" | tr '[:upper:]' '[:lower:]' | tr -d ' ')
      local domain="${DOMAINS[$(($RANDOM % num_domains))]}"
      echo "\"${email_name}@${domain}\"" ;;
    phone)
      echo "\"09$(($RANDOM % 10))$(($RANDOM % 10))-$(($RANDOM % 900 + 100))-$(($RANDOM % 9000 + 1000))\"" ;;
    city)
      echo "\"${CITIES[$(($RANDOM % num_cities))]}\"" ;;
    address)
       local street_name="${STREETS[$(($RANDOM % num_streets))]}"
       local street_num=$(($RANDOM % 1000 + 1))
       echo "\"${street_num} ${street_name}\"" ;;
    number)
      local min=$(echo $field_info | cut -d: -f3); local max=$(echo $field_info | cut -d: -f4)
      local range=$(($max - $min + 1)); echo "$(($RANDOM % $range + $min))" ;;
    boolean)
      if [ $(($RANDOM % 2)) -eq 0 ]; then echo "false"; else echo "true"; fi ;;
    date)
      local days_ago=$(($RANDOM % 730))
      echo "\"$(date -d "-${days_ago} days" --iso-8601=seconds)\"" ;;
    status)
      echo "\"${STATUS_OPTIONS[$(($RANDOM % num_status))]}\"" ;;
    category)
      echo "\"${CATEGORIES[$(($RANDOM % num_categories))]}\"" ;;
    tags)
      local num_chosen_tags=$(($RANDOM % 3 + 1)); local chosen_tags_str="["
      local temp_tags=("${TAGS[@]}"); local num_t=${#temp_tags[@]}
      for (( k=$num_t-1 ; k>0 ; k-- )); do local j=$((RANDOM % (k + 1))); local tmp=${temp_tags[k]}; temp_tags[k]=${temp_tags[j]}; temp_tags[j]=$tmp; done
      for (( k=0; k<$num_chosen_tags; k++ )); do chosen_tags_str+="\"${temp_tags[k]}\""; if [ $k -lt $(($num_chosen_tags - 1)) ]; then chosen_tags_str+=","; fi; done
      echo "$chosen_tags_str]" ;;
    words)
      local num_gen_words=$(($RANDOM % 8 + 3)); local sentence=""
      for (( k=0; k<$num_gen_words; k++ )); do sentence+="${WORDS[$(($RANDOM % num_words))]} "; done
      echo "\"${sentence% }\"" ;;
    *)
      echo "\"unknown_type_${field_type}\"" ;;
  esac
}

# --- Vòng lặp chính để tạo collections ---
created_collections=()
for (( c=1; c<=$COLLECTION_COUNT; c++ )); do

  # Chọn tên collection ngẫu nhiên và đảm bảo không trùng (Giữ nguyên)
  collection_name=""
  while true; do
      collection_name="${POSSIBLE_COLLECTIONS[$(($RANDOM % num_possible_collections))]}"
      is_duplicate=false
      for existing_col in "${created_collections[@]}"; do if [[ "$existing_col" == "$collection_name" ]]; then is_duplicate=true; break; fi; done
      if ! $is_duplicate; then created_collections+=("$collection_name"); break; fi
      if [ ${#created_collections[@]} -eq ${num_possible_collections} ]; then echo "All possible collections generated."; exit 0; fi
  done

  # Quyết định số record và số field ngẫu nhiên (Giữ nguyên)
  record_count=$(($RANDOM % ($MAX_RECORDS - $MIN_RECORDS + 1) + $MIN_RECORDS))
  field_count=$(($RANDOM % ($MAX_FIELDS - $MIN_FIELDS + 1) + $MIN_FIELDS))

  echo "Generating ${record_count} records for collection '${collection_name}' with ~${field_count} fields..."

  # Chọn ngẫu nhiên các field cho collection này (Giữ nguyên)
  selected_field_indices=()
  selected_fields_info=()
  while [ ${#selected_field_indices[@]} -lt $field_count ]; do
      random_index=$(($RANDOM % $num_possible_fields)); is_duplicate=false
      for index in "${selected_field_indices[@]}"; do if [[ $index -eq $random_index ]]; then is_duplicate=true; break; fi; done
      if ! $is_duplicate; then selected_field_indices+=($random_index); selected_fields_info+=("${POSSIBLE_FIELDS[$random_index]}"); fi
  done

  # --- CẬP NHẬT: Tạo chuỗi JSON và gửi bằng curl ---
  json_payload="[" # Bắt đầu mảng JSON lớn

  # Vòng lặp tạo records
  for (( i=1; i<=$record_count; i++ )); do

    # Bắt đầu document JSON
    json_doc="{\"_id\":\"${collection_name:0:3}${i}\""

    # Thêm các field đã chọn ngẫu nhiên
    for field_info in "${selected_fields_info[@]}"; do
      field_name=$(echo $field_info | cut -d: -f1)
      fake_data=$(generate_fake_data "$field_info")
      json_doc+=",\"${field_name}\":${fake_data}"
    done

    # Đóng document JSON
    json_doc+="}"
    json_payload+="$json_doc" # Thêm doc vào payload lớn

    # Thêm dấu phẩy nếu không phải record cuối
    if [ $i -lt $record_count ]; then
      json_payload+=","
    fi
  done

  json_payload+="]" # Đóng mảng JSON lớn

  # Gửi payload bằng curl
  echo "Sending ${record_count} records to ${API_ENDPOINT}/${collection_name}/_insertMany ..."
  curl -X POST -H "Content-Type: application/json" -d "$json_payload" "${API_ENDPOINT}/${collection_name}/_insertMany"
  echo # Thêm dòng mới sau output của curl
  
  echo "--- Finished collection ${collection_name} ---"

done

echo "All data generation complete."