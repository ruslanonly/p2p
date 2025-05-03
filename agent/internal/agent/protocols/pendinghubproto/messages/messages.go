package messages

import (
	"encoding/json"
)

type MessageType string

const (
	TryConnectToMeMessageType MessageType = "TryConnectToMeMessageType"
)

type Message struct {
	Type MessageType     `json:"type"`
	Body json.RawMessage `json:"body"`
}
