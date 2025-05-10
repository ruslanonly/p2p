package classifier

import (
	"fmt"
	"threats/internal/classifier/model"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func buildKDDFlag(tcp *layers.TCP) string {
	switch {
	case tcp.SYN && tcp.ACK:
		return "SF"
	case tcp.SYN && !tcp.ACK:
		return "S0"
	case tcp.RST:
		return "REJ"
	case tcp.FIN:
		return "S1"
	default:
		return "OTH"
	}
}

func mapPortToService(port int) string {
	switch port {
	case 20, 21:
		return "ftp"
	case 22:
		return "ssh"
	case 23:
		return "telnet"
	case 25:
		return "smtp"
	case 53:
		return "domain"
	case 80:
		return "http"
	case 110:
		return "pop3"
	case 143:
		return "imap"
	case 443:
		return "https"
	default:
		return "other"
	}
}

func ExtractTCPIPParameters(packet gopacket.Packet) (*model.ClassificationParameters, error) {
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return nil, fmt.Errorf("IPv4 слой не найден")
	}
	ip, _ := ipLayer.(*layers.IPv4)

	protocol := "unknown"
	service := "unknown"
	flag := "OTH"
	srcBytes := 0
	dstBytes := 0
	urgent := 0

	switch ip.Protocol {
	case layers.IPProtocolTCP:
		tcpLayer := packet.Layer(layers.LayerTypeTCP)
		if tcpLayer == nil {
			return nil, fmt.Errorf("TCP слой не найден")
		}
		tcp, _ := tcpLayer.(*layers.TCP)
		protocol = "tcp"
		flag = buildKDDFlag(tcp)
		srcBytes = len(tcp.Payload)
		urgent = int(tcp.Urgent)
		service = mapPortToService(int(tcp.DstPort))
	case layers.IPProtocolUDP:
		udpLayer := packet.Layer(layers.LayerTypeUDP)
		if udpLayer == nil {
			return nil, fmt.Errorf("UDP слой не найден")
		}
		udp, _ := udpLayer.(*layers.UDP)
		protocol = "udp"
		srcBytes = len(udp.Payload)
		service = mapPortToService(int(udp.DstPort))
	case layers.IPProtocolICMPv4:
		protocol = "icmp"
		service = "icmp"
	}

	land := 0
	if ip.SrcIP.Equal(ip.DstIP) {
		tcp := packet.Layer(layers.LayerTypeTCP)
		if tcp != nil {
			tcpLayer := tcp.(*layers.TCP)
			if tcpLayer.SrcPort == tcpLayer.DstPort {
				land = 1
			}
		}
	}

	p := &model.ClassificationParameters{
		Duration:      float32(0),
		ProtocolType:  protocol,
		Service:       service,
		Flag:          flag,
		SrcBytes:      float32(srcBytes),
		DstBytes:      float32(dstBytes),
		Land:          land,
		WrongFragment: int(ip.FragOffset),
		Urgent:        urgent,

		// Остальные параметры требуют агрегации по потоку или анализ логина:
		Hot:                    0,
		NumFailedLogins:        0,
		LoggedIn:               0,
		NumCompromised:         0,
		RootShell:              0,
		SuAttempted:            0,
		NumRoot:                0,
		NumFileCreations:       0,
		NumShells:              0,
		NumAccessFiles:         0,
		NumOutboundCmds:        0,
		IsHostLogin:            0,
		IsGuestLogin:           0,
		Count:                  1,
		SrvCount:               1,
		SerrorRate:             0,
		SrvSerrorRate:          0,
		RerrorRate:             0,
		SrvRerrorRate:          0,
		SameSrvRate:            0,
		DiffSrvRate:            0,
		SrvDiffHostRate:        0,
		DstHostCount:           0,
		DstHostSrvCount:        0,
		DstHostSameSrvRate:     0,
		DstHostDiffSrvRate:     0,
		DstHostSameSrcPortRate: 0,
		DstHostSrvDiffHostRate: 0,
		DstHostSerrorRate:      0,
		DstHostSrvSerrorRate:   0,
		DstHostRerrorRate:      0,
		DstHostSrvRerrorRate:   0,
	}

	return p, nil
}
