package sniffer

import (
	"fmt"
	"threats/internal/classifier/model"
	"time"

	"github.com/CN-TU/go-flows/flows"
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

	packetSource := gopacket.NewPacketSource(s.handle, s.handle.LinkType())

	options := flows.FlowOptions{
		ActiveTimeout: 1 * flows.SecondsInNanoseconds,
		IdleTimeout:   1 * flows.SecondsInNanoseconds,
		TCPExpiry:     true,
		WindowExpiry:  true,
	}

	fiveTuple := false
	records := flows.RecordListMaker{}

	id := uint8(0)
	flowTable := flows.NewFlowTable(
		records,
		func(e flows.Event, t *flows.FlowTable, k string, a bool, ctx *flows.EventContext, id uint64) flows.Flow {
			return NewCICIDSFlow(e, t, k, a, ctx, id, handler)
		},
		options,
		fiveTuple,
		id,
	)

	i := uint64(0)
	for packet := range packetSource.Packets() {
		length := packet.Metadata().Length
		key := GenerateKey(packet)
		lowToHigh := DetermineDirection(packet)

		event := &PacketEvent{
			timestamp: flows.DateTimeNanoseconds(time.Now().UnixNano()),
			key:       key,
			lowToHigh: lowToHigh,
			eventNr:   i,
			length:    length,
			Packet:    packet,
		}

		flowTable.Event(event)
		i++
	}

	fmt.Println("END OF SNIFFING")
}
