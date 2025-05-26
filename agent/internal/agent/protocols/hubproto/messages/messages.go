package messages

import (
	"net"

	"github.com/libp2p/go-libp2p/core/peer"
	statusmodel "github.com/ruslanonly/agent/internal/agent/model/status"
)

type MessageType string

const (
	RedTrafficMessageType    MessageType = "RedTrafficMessageType"
	YellowTrafficMessageType MessageType = "YellowTrafficMessageType"
	InfoAboutHubMessageType  MessageType = "InfoAboutHubMessageType"
)

type MessageBody []byte

type Message struct {
	FromID  peer.ID     `json:"from_id"`
	Type    MessageType `json:"type"`
	Body    MessageBody `json:"body"`
	Visited []peer.ID   `json:"visited"`
}

type RedTrafficMessageBody net.IP

type InfoAboutHubMessageBody struct {
	ID     string                     `json:"id"`
	Addrs  []string                   `json:"addrs"`
	Status statusmodel.HubSlotsStatus `json:"status"`
}
