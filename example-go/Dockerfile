# Build stage
FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/api

# Runtime stage
FROM alpine:3.22

WORKDIR /app

COPY --from=builder /server ./server

EXPOSE 8090

CMD ["./server"]