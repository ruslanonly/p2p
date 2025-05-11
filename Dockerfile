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

WORKDIR /app

# Установка зависимостей
RUN apt-get update && \
    apt-get install -y iptables iproute2 net-tools curl ca-certificates libpcap-dev wget && \
    wget https://github.com/microsoft/onnxruntime/releases/download/v1.22.0/onnxruntime-linux-aarch64-1.22.0.tgz && \
    tar -xzf onnxruntime-linux-aarch64-1.22.0.tgz && \
    cp onnxruntime-linux-aarch64-1.22.0/lib/libonnxruntime.so* /usr/lib/ && \
    cp /usr/lib/libonnxruntime.so /app/onnxruntime.so && \
    ln -s /usr/lib/libonnxruntime.so /usr/lib/onnxruntime.so && \
    rm -rf onnxruntime-linux-aarch64-1.22.0* && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

ENV LD_LIBRARY_PATH=/usr/lib

# Копируем бинарники
COPY --from=agent-builder /out/agent /app/agent
COPY --from=threats-builder /out/threats /app/threats
COPY ./ml/model.onnx /app/model.onnx
COPY ./ml/*.json /app/

# Копируем скрипт запуска
COPY ./start.sh /app/start.sh
RUN chmod +x /app/agent /app/threats /app/start.sh

CMD ["/app/start.sh"]
