# ==============================
# 🧩 Build stage
# ==============================
FROM golang:1.22 AS builder
WORKDIR /app

# Copy dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# ✅ Build native cho ARM64 (Mac M1)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o MiniDBGo ./cmd/MiniDBGo

# ==============================
# 🚀 Runtime stage
# ==============================
FROM debian:bookworm-slim
WORKDIR /app

# Install curl for healthcheck / debug
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/MiniDBGo /usr/local/bin/MiniDBGo
RUN mkdir -p /data/MiniDBGo && chmod -R 777 /data

EXPOSE 6866

ENV MODE=server
CMD ["MiniDBGo"]
