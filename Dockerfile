# =========================
# Build stage
# =========================
FROM golang:1.25.4 AS builder

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOMAXPROCS=1

WORKDIR /app

# Copy only module files first
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with reduced memory usage
RUN go build -trimpath -ldflags="-s -w" -o server

# =========================
# Runtime stage
# =========================
FROM registry.access.redhat.com/ubi9/ubi-micro

WORKDIR /app
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["/app/server"]
