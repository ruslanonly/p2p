# 🧱 Stage 1: Сборка agent
FROM golang:1.24 AS agent-builder
WORKDIR /agent
COPY ./agent/go.* ./
RUN go mod download
COPY ./agent .
RUN go build -o /out/agent ./main.go

# 🧱 Stage 2: Сборка pcap
FROM golang:1.24 AS pcap-builder
WORKDIR /pcap
COPY ./pcap/go.* ./
RUN go mod download
COPY ./pcap .
RUN go build -o /out/pcap ./main.go

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
COPY --from=pcap-builder /out/pcap /app/pcap

# Копируем скрипт запуска
COPY ./start.sh /app/start.sh
RUN chmod +x /app/agent /app/pcap /app/start.sh

CMD ["/app/start.sh"]
