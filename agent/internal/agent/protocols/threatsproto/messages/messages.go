package messages

import (
	"net"
)

type MessageType string

const (
	// Отправляет абонент хабу
	RedTrafficMessageType MessageType = "RedTrafficMessageType"
	// Отправляет абонент хабу
	YellowTrafficMessageType MessageType = "YellowTrafficMessageType"
	// Отправляет хаб абоненту
	BlockTrafficMessageType MessageType = "BlockHostMessageType"
)

type Message struct {
	Type MessageType `json:"type"`
	IP   net.IP      `json:"ip"`
}
