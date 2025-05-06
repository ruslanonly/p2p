package model

import "net"

type TCPIPClassificationParameters struct {
	SrcIP net.IP
	DstIP net.IP
	SYN   bool
}

type UDPClassificationParameters struct {
	SrcIP   net.IP
	DstIP   net.IP
	SrcPort uint16
	DstPort uint16
	Length  uint16
}
