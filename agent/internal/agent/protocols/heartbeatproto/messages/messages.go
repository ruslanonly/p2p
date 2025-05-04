package messages

type MessageType string

const (
	CheckHeartbeatMessageType MessageType = "CheckHeartbeatMessageType"
	BoomBoomMessageType       MessageType = "BoomBoomMessageType"
)

type Message struct {
	Type MessageType `json:"type"`
}
