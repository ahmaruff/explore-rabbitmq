# Build stage
FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build publisher binary
RUN CGO_ENABLED=0 go build -o /pub ./cmd/pub

# Build subscriber binary
RUN CGO_ENABLED=0 go build -o /sub ./cmd/sub


# Publisher runtime image
FROM alpine:3.22 AS pub

WORKDIR /app

COPY --from=builder /pub ./pub

CMD ["./pub"]


# Subscriber runtime image
FROM alpine:3.22 AS sub

WORKDIR /app

COPY --from=builder /sub ./sub

CMD ["./sub"]