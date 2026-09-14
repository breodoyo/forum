FROM golang:alpine AS builder

RUN apk add --no-cache build-base

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o forum ./cmd/server

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/forum .
COPY --from=builder /app/web ./web

RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./forum"]