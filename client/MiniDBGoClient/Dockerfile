# ==============================
# 🧩 Build stage
# ==============================
FROM node:20-alpine AS builder
WORKDIR /client

COPY package.json yarn.lock ./
RUN yarn install --frozen-lockfile

COPY . .
RUN yarn build

# ==============================
# 🚀 Runtime stage
# ==============================
FROM nginx:alpine
COPY --from=builder /client/dist /usr/share/nginx/html

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
