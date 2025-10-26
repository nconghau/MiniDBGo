# --- STAGE 1: Build ---
# Sử dụng một image Node.js LTS (Long Term Support) phiên bản Alpine (nhỏ gọn)
FROM node:20-alpine AS builder

# Thiết lập thư mục làm việc bên trong container
WORKDIR /app

# Sao chép package.json
COPY package.json .

# --- SỬA LỖI: Thêm --legacy-peer-deps ---
# Bỏ qua lỗi không tương thích peer dependency của react-json-view
RUN npm install --legacy-peer-deps

# Sao chép toàn bộ mã nguồn còn lại
COPY . .

# Đây là biến build-time sẽ được truyền từ docker-compose.yml
# Nó sẽ chứa URL của backend Go, ví dụ: http://localhost:6866/api
ARG PUBLIC_BASE_URL_ARG

# Khi chạy lệnh build, chúng ta gán biến ARG ở trên vào biến ENV
# mà rsbuild (hoặc Vite/CRA) sẽ đọc để "bake" vào code JavaScript
# Tên biến PUBLIC_BASE_URL phải khớp với file .env của bạn (PUBLIC_...)
RUN PUBLIC_BASE_URL=${PUBLIC_BASE_URL_ARG} npm run build

# --- STAGE 2: Production ---
# Sử dụng image Nginx chính thức, phiên bản Alpine (siêu nhỏ)
FROM nginx:stable-alpine

# Xóa cấu hình Nginx mặc định
RUN rm /etc/nginx/conf.d/default.conf

# Sao chép tệp cấu hình Nginx tùy chỉnh của chúng ta (nginx.conf)
# Tệp này sẽ xử lý việc phục vụ SPA
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Sao chép các file đã build (từ thư mục /app/dist của tầng builder)
# vào thư mục phục vụ web mặc định của Nginx
COPY --from=builder /app/dist /usr/share/nginx/html

# Expose cổng 80 mà Nginx đang lắng nghe bên trong container
EXPOSE 80

# Lệnh để khởi động Nginx ở chế độ foreground
CMD ["nginx", "-g", "daemon off;"]
