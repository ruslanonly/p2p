# 🧱 Stage 1: Сборка Go-приложения
FROM golang:1.23 AS builder

WORKDIR /app

# Копируем только go.mod и go.sum для установки зависимостей
COPY ./app/go.mod ./app/go.sum ./

# Устанавливаем зависимости и кешируем их
RUN go mod download

# Теперь копируем весь остальной код
COPY ./app .

# Собираем бинарник
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o agent ./main.go

# 📦 Stage 2: Финальный образ
FROM ubuntu:22.04

# Установка зависимостей (iptables, net-tools и т.п.)
RUN apt-get update && \
    apt-get install -y iptables iproute2 net-tools curl ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Копируем бинарник из билдера
COPY --from=builder /app/agent .

# Даём права на выполнение
RUN chmod +x ./agent

# Запуск
CMD ["./agent"]
