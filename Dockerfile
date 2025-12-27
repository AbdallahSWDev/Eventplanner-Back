# =============== Build stage ===============
FROM golang:1.25 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./...

# =============== Final stage ===============
FROM registry.access.redhat.com/ubi9/ubi-micro:latest

WORKDIR /app

COPY --from=builder /src/server .

EXPOSE 8080

CMD ["/app/server"]
