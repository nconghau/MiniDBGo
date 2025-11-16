# --- STAGE 1: Build ---
# SỬA LỖI: Dùng image Node.js đầy đủ (glibc) thay vì Alpine (musl)
FROM node:20 AS builder

# Thiết lập thư mục làm việc bên trong container
WORKDIR /app

# Sao chép package.json và package-lock.json
COPY package.json package-lock.json ./

# Bỏ qua lỗi không tương thích peer dependency của react-json-view
RUN npm install --legacy-peer-deps

# Sao chép toàn bộ mã nguồn còn lại
COPY . .

# Đây là biến build-time sẽ được truyền từ docker-compose.yml
ARG PUBLIC_BASE_URL

# Chạy lệnh build
RUN PUBLIC_BASE_URL=${PUBLIC_BASE_URL} npm run build

# --- STAGE 2: Production ---
# Sử dụng image Nginx chính thức, phiên bản Alpine (siêu nhỏ)
# Giai đoạn này dùng Alpine là TỐT, vì nó chỉ phục vụ file tĩnh
FROM nginx:stable-alpine

# Xóa cấu hình Nginx mặc định
RUN rm /etc/nginx/conf.d/default.conf

# Sao chép tệp cấu hình Nginx tùy chỉnh của chúng ta (nginx.conf)
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Sao chép các file đã build (từ thư mục /app/dist của tầng builder)
COPY --from=builder /app/dist /usr/share/nginx/html

# Expose cổng 80 mà Nginx đang lắng nghe bên trong container
EXPOSE 80

# Lệnh để khởi động Nginx ở chế độ foreground
CMD ["nginx", "-g", "daemon off;"]