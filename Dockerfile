# =========================
# Build stage
# =========================
FROM registry.access.redhat.com/ubi9/go-toolset:1.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server

# =========================
# Runtime stage
# =========================
FROM registry.access.redhat.com/ubi9/ubi-micro:latest

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["/app/server"]
