# =========================
# Build stage
# =========================
FROM golang:1.25.4 AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GO111MODULE=on \
    GOPROXY=https://proxy.golang.org,direct

WORKDIR /app

# Copy module files
COPY go.mod go.sum ./

# IMPORTANT: do NOT pre-download modules
# Let go build resolve everything in one step

COPY . .

# Single-step build with verbose output
RUN go build -v -mod=mod -o server

# =========================
# Runtime stage
# =========================
FROM registry.access.redhat.com/ubi9/ubi-micro:latest

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["/app/server"]
