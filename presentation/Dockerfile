FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY ./go.mod ./go.mod
COPY ./go.sum ./go.sum
COPY . .

RUN go mod tidy
RUN go build -o server

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .
CMD ["./server"]