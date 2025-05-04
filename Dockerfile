# 🧱 Stage 1: Сборка agent
FROM golang:1.24 AS agent-builder
WORKDIR /agent
COPY ./agent/go.* ./
COPY ./pkg ../pkg
RUN go mod download
COPY ./agent .
RUN go build -o /out/agent ./main.go

# 🧱 Stage 2: Сборка traffic
FROM golang:1.24 AS traffic-builder
WORKDIR /traffic
COPY ./traffic/go.* ./
COPY ./pkg ../pkg
RUN go mod download
COPY ./traffic .
RUN go build -o /out/traffic ./main.go

# 📦 Stage 3: Финальный минимальный образ
FROM ubuntu:25.04

# Установка зависимостей
RUN apt-get update && \
    apt-get install -y iptables iproute2 net-tools curl ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Копируем бинарники
COPY --from=agent-builder /out/agent /app/agent
COPY --from=traffic-builder /out/traffic /app/traffic

# Копируем скрипт запуска
COPY ./start.sh /app/start.sh
RUN chmod +x /app/agent /app/traffic /app/start.sh

CMD ["/app/start.sh"]
