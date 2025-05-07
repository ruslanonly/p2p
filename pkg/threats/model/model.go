package model

import "net"

type MessageType int

var YellowTrafficMessageType MessageType = 333
var RedTrafficMessageType MessageType = 444
var BlockHostMessageType MessageType = 5551
var MercyHostMessageType MessageType = 5552

type RedTrafficMessageTypeBody struct {
	IP net.IP `json:"ip"`
}

type YellowTrafficMessageTypeBody struct {
	IP net.IP `json:"ip"`
}

type BlockHostMessageTypeBody struct {
	IP net.IP `json:"ip"`
}

type MercyHostMessageTypeBody struct {
	IP net.IP `json:"ip"`
}
