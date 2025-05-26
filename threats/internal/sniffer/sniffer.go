package sniffer

import (
	"fmt"
	"threats/internal/classifier/model"
	"threats/internal/sniffer/flow"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

type Sniffer struct {
	handle *pcap.Handle
}

func NewSniffer(iface string) (*Sniffer, error) {
	snaplen := int32(1600)
	promisc := false
	timeout := pcap.BlockForever

	handle, err := pcap.OpenLive(iface, snaplen, promisc, timeout)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия интерфейса: %w", err)
	}

	return &Sniffer{handle: handle}, nil
}

func (s *Sniffer) Run(handler func(*model.TrafficParameters)) {
	defer s.handle.Close()

	flowsMngr := flow.NewFlowsManager(handler)

	packetSource := gopacket.NewPacketSource(s.handle, s.handle.LinkType())
	for packet := range packetSource.Packets() {
		event := flow.FlowEvent{
			Packet: packet,
		}

		flowsMngr.AddEvent(event)
	}
}
