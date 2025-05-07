package messages

import (
	"net"

	"github.com/libp2p/go-libp2p/core/peer"
)

type MessageType string

const (
	RedTrafficMessageType MessageType = "RedTrafficMessageType"
)

type Message struct {
	FromID  peer.ID     `json:"from_id"`
	Type    MessageType `json:"type"`
	Payload []byte      `json:"body"`
	Visited []peer.ID   `json:"visited"`
}

type RedTrafficMessageBody net.IP
