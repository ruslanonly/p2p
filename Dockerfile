# 🧱 Stage 1: Сборка agent
FROM golang:1.24 AS agent-builder
WORKDIR /agent
COPY ./agent/go.* ./
COPY ./pkg ../pkg
RUN go mod download
COPY ./agent .
RUN go build -o /out/agent ./main.go

# 🧱 Stage 2: Сборка threats
FROM golang:1.24 AS threats-builder
RUN apt-get update && apt-get install -y libpcap-dev

WORKDIR /threats
COPY ./threats/go.* ./
COPY ./pkg ../pkg
RUN go mod download
COPY ./threats .
RUN go build -o /out/threats ./main.go

# 📦 Stage 3: Финальный минимальный образ
FROM ubuntu:25.04

# Установка зависимостей
RUN apt-get update && \
    apt-get install -y iptables iproute2 net-tools curl ca-certificates libpcap-dev && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Копируем бинарники
COPY --from=agent-builder /out/agent /app/agent
COPY --from=threats-builder /out/threats /app/threats

# Копируем скрипт запуска
COPY ./start.sh /app/start.sh
RUN chmod +x /app/agent /app/threats /app/start.sh

CMD ["/app/start.sh"]
