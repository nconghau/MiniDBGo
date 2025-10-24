# ============================================
# STAGE 1 — Build binary
# ============================================
FROM golang:1.22-alpine AS builder

# Install git (needed for go modules)
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY go.mod ./
# Copy source code
COPY . .

# Download dependencies
RUN go mod tidy

# Build static binary (no CGO)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/minidb ./cmd/MiniDBGo

# ============================================
# STAGE 2 — Run minimal image
# ============================================
FROM alpine:latest

# Set working directory
WORKDIR /app

# Copy compiled binary from builder
COPY --from=builder /bin/minidb /usr/local/bin/minidb

# Create default data directory
RUN mkdir -p /app/data/MiniDBGo

# Set environment variables
ENV PATH="/usr/local/bin:$PATH"

# Optional: expose nothing (CLI app) or port if needed in future
# EXPOSE 6866

# Run as non-root user for security
RUN adduser -D minidbuser
USER minidbuser

# Default working directory (inside container)
WORKDIR /app

# Run CLI by default
ENTRYPOINT ["minidb"]
