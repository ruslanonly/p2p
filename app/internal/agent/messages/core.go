package messages

import (
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/peer"
)

type MessageType string

const (
	ConnectRequestMessageType            MessageType = "ConnectRequestMessageType"
	ConnectedMessageType                 MessageType = "ConnectedMessageType"
	NotConnectedMessageType              MessageType = "NotConnectedMessageType"
	NotConnectedAndWaitMessageType       MessageType = "NotConnectedAndWaitMessageType"
	InitializeElectionRequestMessageType MessageType = "InitializeElectionRequestMessageType"
	BecomeOnlyOneHubMessageType          MessageType = "BecomeOnlyOneHubMessageType"
	InfoAboutMeForHubsMessageType        MessageType = "InfoAboutMeForHubsMessageType"
	InfoAboutSegmentMessageType          MessageType = "InfoAboutSegmentMessageType"

	ElectionRequestMessageType MessageType = "ElectionRequestMessageType"
)

type Message struct {
	Type MessageType     `json:"type"`
	Body json.RawMessage `json:"body"`
}

type ConnectRequestMessageBody struct {
	ID string `json:"ip"`
}

type HubSlotsStatus string

const (
	// Есть свободные слоты для подключения
	FreeHubSlotsStatus HubSlotsStatus = "FreeHubSlotsStatus"
	// Занят, но есть абоненты для инициализации выборов
	FullHavingAbonentsHubSlotsStatus HubSlotsStatus = "FullHavingAbonentsHubSlotsStatus"
	// Полностью занят
	FullNotHavingAbonentsHubSlotsStatus HubSlotsStatus = "FullNotHavingAbonentsHubSlotsStatus"
)

type InfoAboutMeForHubsMessageBody struct {
	ID     string         `json:"id"`
	Addrs  string         `json:"addrs"`
	Status HubSlotsStatus `json:"status"`
}

type InfoAboutSegmentPeerInfo struct {
	ID    peer.ID  `json:"id"`
	Addrs []string `json:"addrs"`
	IsHub bool     `json:"is_hub"`
}

type ConnectedMessageBody struct {
	Peers []InfoAboutSegmentPeerInfo `json:"peers"`
}

type InitializeElectionRequestMessageBody struct {
	Peers []InfoAboutSegmentPeerInfo `json:"peers"`
}

type ElectionRequestMessageBody struct {
	Peers []InfoAboutSegmentPeerInfo `json:"peers"`
}

type InfoAboutSegmentMessageBody struct {
	Peers []InfoAboutSegmentPeerInfo `json:"peers"`
}
