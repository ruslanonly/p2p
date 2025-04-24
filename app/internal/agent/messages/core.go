package messages

import "encoding/json"

type MessageType string

const (
	ConnectRequestMessageType   MessageType = "ConnectRequestMessageType"
	ConnectedMessageType   MessageType = "ConnectedMessageType"
	NotConnectedMessageType   MessageType = "NotConnectedMessageType"
	InfoAboutMeMessageType MessageType = "InfoAboutMeMessageType"

	DiscoverPeersMessageType    MessageType = "DISCOVER_PEERS"
	FindSuperpeerMessageType    MessageType = "FIND_SUPERPEER"
	FindFreeSuperpeerMessageType        MessageType = "FIND_FREE_SUPERPEER"
	FreeSuperpeerMessageType            MessageType = "FREE_SUPERPEER"
	FreeSuperpeerForBootstrapMessageType MessageType = "FREE_SUPERPEER_FOR_BOOTSTRAP"
	ElectNewSuperpeerMessageType        MessageType = "ELECT_NEW_SUPERPEER"
	CandidateSuperpeerMessageType       MessageType = "CANDIDATE_SUPERPEER"
	ReportPeersMessageType       		MessageType = "REPORT_PEERS"
)

type Message struct {
	Type MessageType     `json:"type"`
	Body json.RawMessage `json:"body"`
}

type ConnectRequestMessageBody struct {
	IP string `json:"ip"`
}

type PeerInfo struct {
	IP       string      `json:"ip"`
	IsSuper  bool        `json:"is_super"`
	Peers    []PeerInfo  `json:"peers,omitempty"`
}

type DiscoverPeersResponse = []PeerInfo
type ReportPeersRequest = []PeerInfo

type FreeSuperpeerForBootstrap struct {
	IP string `json:"ip"`
}

type FreeSuperpeer struct {
	IP string `json:"ip"`
}