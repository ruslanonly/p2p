package model

type MessageType int

var YellowTrafficMessageType MessageType = 333
var RedTrafficMessageType MessageType = 444
var BlockHostMessageType MessageType = 555

type RedTrafficMessageTypeBody struct {
	IP string `json:"ip"`
}

type YellowTrafficMessageTypeBody struct {
	IP string `json:"ip"`
}

type BlockHostMessageTypeBody struct {
	IP string `json:"ip"`
}
