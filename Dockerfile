# ==============================
# 🧩 Build stage
# ==============================
FROM golang:1.22 AS builder
WORKDIR /app

# Copy go.mod and download deps
COPY go.mod go.sum ./
RUN go mod download

# Copy full source
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o MiniDBGo ./cmd/MiniDBGo

# ==============================
# 🚀 Runtime stage
# ==============================
FROM alpine:latest
WORKDIR /app

# Copy built binary
COPY --from=builder /app/MiniDBGo /usr/local/bin/MiniDBGo

# Create data dir and fix permissions
RUN mkdir -p /data/MiniDBGo && chmod -R 777 /data

# Expose port
EXPOSE 6866

# Healthcheck (check API is alive)
HEALTHCHECK --interval=10s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:6866/api/_collections || exit 1

# Final run command
CMD ["sh", "-c", "mkdir -p /data/MiniDBGo && chmod -R 777 /data && MiniDBGo"]
