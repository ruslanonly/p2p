package messages

import (
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/peer"
)

type MessageType string

const (
	ConnectRequestMessageType            MessageType = "ConnectRequestMessageType"
	ConnectRequestMessageAsHubType       MessageType = "ConnectRequestMessageAsHubType"
	ConnectedMessageType                 MessageType = "ConnectedMessageType"
	NotConnectedMessageType              MessageType = "NotConnectedMessageType"
	NotConnectedAndWaitMessageType       MessageType = "NotConnectedAndWaitMessageType"
	InitializeElectionRequestMessageType MessageType = "InitializeElectionRequestMessageType"
	BecomeOnlyOneHubMessageType          MessageType = "BecomeOnlyOneHubMessageType"
	InfoAboutSegmentMessageType          MessageType = "InfoAboutSegmentMessageType"
	ElectionRequestMessageType           MessageType = "ElectionRequestMessageType"

	// Узел отправляет хабу в случае, когда ему необходимо уведомить хаба об отключении
	DisconnectMessageType MessageType = "DisconnectMessageType"
)

type Message struct {
	Type MessageType     `json:"type"`
	Body json.RawMessage `json:"body"`
}

type NotConnectedMessageBody struct {
	ID    peer.ID  `json:"id"`
	Addrs []string `json:"addrs"`
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
