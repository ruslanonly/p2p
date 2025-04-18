# 🧱 Stage 1: сборка Go-приложения
FROM golang:1.23 AS builder

WORKDIR /app

COPY ./app .

RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o agent ./main.go

FROM ubuntu:22.04

# Установка зависимостей (iptables, net-tools, и т.п.)
RUN apt-get update && \
    apt-get install -y iptables iproute2 net-tools curl ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Копируем бинарник из предыдущего этапа
COPY --from=builder /app/agent .

# Даём права на выполнение
RUN chmod +x ./agent

# Запуск
CMD ["./agent"]
