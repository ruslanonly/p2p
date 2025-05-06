package classifier

import (
	"fmt"
	"threats/internal/classifier/model"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func ExtractUDPParameters(packet gopacket.Packet) (*model.UDPClassificationParameters, error) {
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	udpLayer := packet.Layer(layers.LayerTypeUDP)

	if ipLayer != nil && udpLayer != nil {
		ip, _ := ipLayer.(*layers.IPv4)
		udp, _ := udpLayer.(*layers.UDP)

		p := &model.UDPClassificationParameters{
			SrcIP:   ip.SrcIP,
			DstIP:   ip.DstIP,
			SrcPort: uint16(udp.SrcPort),
			DstPort: uint16(udp.DstPort),
			Length:  udp.Length,
		}

		return p, nil
	} else {
		return nil, fmt.Errorf("возникла ошибка при обработке UDP слоев gopacket")
	}
}

func ExtractTCPIPParameters(packet gopacket.Packet) (*model.TCPIPClassificationParameters, error) {
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	tcpLayer := packet.Layer(layers.LayerTypeTCP)

	if ipLayer != nil && tcpLayer != nil {
		ip, _ := ipLayer.(*layers.IPv4)
		tcp, _ := tcpLayer.(*layers.TCP)

		p := &model.TCPIPClassificationParameters{
			SrcIP: ip.SrcIP,
			DstIP: ip.DstIP,
			SYN:   tcp.SYN,
		}

		return p, nil
	} else {
		return nil, fmt.Errorf("возникла ошибка при обработке слоев gopacket")
	}
}
