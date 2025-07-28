# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ ./vendor
ENV GOFLAGS="-mod=vendor"

COPY . .

RUN go build -o users-service ./cmd/main.go

# Stage 2: Run
FROM alpine

WORKDIR /app

COPY --from=builder /app/users-service .

EXPOSE 8083

CMD ["./users-service"]