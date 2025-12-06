FROM golang:1.25.4

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the entire module, not just main.go
RUN go build -o server .

EXPOSE 8080

CMD ["./server"]
