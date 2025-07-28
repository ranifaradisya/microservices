# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY vendor/ ./vendor
ENV GOFLAGS="-mod=vendor"

COPY . .

RUN go build -o products-service ./cmd/main.go

# Stage 2: Run
FROM alpine

WORKDIR /app

COPY --from=builder /app/products-service .

EXPOSE 8081

CMD ["./products-service"]