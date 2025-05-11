package classifier

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"threats/internal/classifier/model"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type Aggregator struct {
	flagMapping         map[string]float32
	protocolTypeMapping map[string]float32
	serviceMapping      map[string]float32
}

func loadMapping(filename string) (map[string]float32, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var raw map[string]interface{}
	err = json.NewDecoder(file).Decode(&raw)
	if err != nil {
		return nil, err
	}

	mapping := make(map[string]float32)
	for k, v := range raw {
		switch num := v.(type) {
		case float64:
			mapping[k] = float32(num)
		default:
			return nil, fmt.Errorf("unexpected type for key %s: %T", k, v)
		}
	}

	return mapping, nil
}

func NewAggregator() (*Aggregator, error) {
	flagMapping, err := loadMapping("flag_mapping.json")
	if err != nil {
		return nil, fmt.Errorf("не удалось получить flag mapping")
	}

	protocolTypeMapping, err := loadMapping("protocol_type_mapping.json")
	if err != nil {
		return nil, fmt.Errorf("не удалось получить protocol type mapping")
	}

	serviceMapping, err := loadMapping("service_mapping.json")
	if err != nil {
		return nil, fmt.Errorf("не удалось получить service mapping")
	}

	aggregator := &Aggregator{
		flagMapping:         flagMapping,
		protocolTypeMapping: protocolTypeMapping,
		serviceMapping:      serviceMapping,
	}

	return aggregator, nil
}

func (a *Aggregator) Extract(packet gopacket.Packet) (*model.TrafficParameters, error) {
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return nil, fmt.Errorf("IPv4 слой не найден")
	}
	ip := ipLayer.(*layers.IPv4)

	protocol := "unknown"
	service := "unknown"
	flag := "OTH"
	srcBytes := 0
	dstBytes := 0
	urgent := 0
	land := 0

	switch ip.Protocol {
	case layers.IPProtocolTCP:
		protocol = "tcp"
	case layers.IPProtocolUDP:
		protocol = "udp"
	case layers.IPProtocolICMPv4:
		protocol = "icmp"
	default:
		protocol = "other"
	}

	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp := tcpLayer.(*layers.TCP)
		flag = getTCPFlag(tcp)
		srcBytes = len(tcp.Payload)
		dstBytes = 0 // Пока что не определяем отдельно
		urgent = int(tcp.Urgent)
		service = a.guessService(uint16(tcp.DstPort)) // на основе порта
		if ip.SrcIP.Equal(ip.DstIP) && tcp.SrcPort == tcp.DstPort {
			land = 1
		}
	} else if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp := udpLayer.(*layers.UDP)
		srcBytes = len(udp.Payload)
		dstBytes = 0
		service = a.guessService(uint16(udp.DstPort))
		if ip.SrcIP.Equal(ip.DstIP) && udp.SrcPort == udp.DstPort {
			land = 1
		}
	}

	hot := 0
	numFailedLogins := 0
	loggedIn := 0
	numCompromised := 0
	rootShell := 0
	suAttempted := 0
	numRoot := 0
	numFileCreations := 0
	numShells := 0
	numAccessFiles := 0
	numOutboundCmds := 0
	isHotLogin := 0
	isGuestLogin := 0

	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {

		tcp := tcpLayer.(*layers.TCP)
		payload := tcp.Payload
		payloadStr := strings.ToUpper(string(payload))

		if strings.Contains(payloadStr, "USER ") {
			if strings.Contains(payloadStr, "ANONYMOUS") || strings.Contains(payloadStr, "GUEST") {
				isGuestLogin = 1
			}
			if strings.Contains(payloadStr, "ROOT") || strings.Contains(payloadStr, "ADMIN") {
				isHotLogin = 1
				hot++
			}
		}

		if strings.Contains(payloadStr, "PASS ") {
			// возможен логин
		}

		if strings.Contains(payloadStr, "230") {
			loggedIn = 1
		} else if strings.Contains(payloadStr, "530") {
			numFailedLogins++
		}

		// Команды компрометации
		if strings.Contains(payloadStr, "SU ROOT") {
			suAttempted = 1
			numCompromised++
			numRoot++
		}

		if strings.Contains(payloadStr, "SITE EXEC") || strings.Contains(payloadStr, "SHELL") {
			numShells++
			numCompromised++
		}

		if strings.Contains(payloadStr, "/ETC/PASSWD") || strings.Contains(payloadStr, "/ETC/SHADOW") {
			numAccessFiles++
			numCompromised++
		}

		// FTP-операции
		if strings.Contains(payloadStr, "MKD") {
			numFileCreations++
		}

		if strings.Contains(payloadStr, "RETR") || strings.Contains(payloadStr, "STOR") {
			numOutboundCmds++
		}
	}

	// Заполнение структуры
	p := &model.TrafficParameters{
		Duration:      float32(packet.Metadata().Timestamp.Second()),
		ProtocolType:  protocol,
		Service:       service,
		Flag:          flag,
		SrcBytes:      float32(srcBytes),
		DstBytes:      float32(dstBytes),
		Land:          land,
		WrongFragment: int(ip.FragOffset),
		Urgent:        urgent,

		Hot:                    hot,
		NumFailedLogins:        numFailedLogins,
		LoggedIn:               loggedIn,
		NumCompromised:         numCompromised,
		RootShell:              rootShell,
		SuAttempted:            suAttempted,
		NumRoot:                numRoot,
		NumFileCreations:       numFileCreations,
		NumShells:              numShells,
		NumAccessFiles:         numAccessFiles,
		NumOutboundCmds:        numOutboundCmds,
		IsHostLogin:            isHotLogin,
		IsGuestLogin:           isGuestLogin,
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

func getTCPFlag(tcp *layers.TCP) string {
	switch {
	case tcp.SYN && !tcp.ACK:
		return "S0"
	case tcp.SYN && tcp.ACK:
		return "S1"
	case tcp.RST:
		return "REJ"
	case tcp.FIN:
		return "SF"
	default:
		return "OTH"
	}
}

func (a *Aggregator) guessService(port uint16) string {
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
	case 443:
		return "https"
	case 445:
		return "microsoft-ds"
	case 3306:
		return "mysql"
	case 1433:
		return "sql"
	default:
		return "other"
	}
}

func (e *Aggregator) Vectorize(p model.TrafficParameters) []float32 {
	getOrDefault := func(m map[string]float32, key string) float32 {
		if val, ok := m[key]; ok {
			return val
		}
		return 0.0
	}
	protocolVal := getOrDefault(e.protocolTypeMapping, p.ProtocolType)
	serviceVal := getOrDefault(e.serviceMapping, p.Service)
	flagVal := getOrDefault(e.flagMapping, p.Flag)

	inputVector := []float32{
		p.Duration,
		protocolVal,
		serviceVal,
		flagVal,
		p.SrcBytes,
		p.DstBytes,
		float32(p.Land),
		float32(p.WrongFragment),
		float32(p.Urgent),
		float32(p.Hot),
		float32(p.NumFailedLogins),
		float32(p.LoggedIn),
		float32(p.NumCompromised),
		float32(p.RootShell),
		float32(p.SuAttempted),
		float32(p.NumRoot),
		float32(p.NumFileCreations),
		float32(p.NumShells),
		float32(p.NumAccessFiles),
		float32(p.NumOutboundCmds),
		float32(p.IsHostLogin),
		float32(p.IsGuestLogin),
		p.Count,
		p.SrvCount,
		p.SerrorRate,
		p.SrvSerrorRate,
		p.RerrorRate,
		p.SrvRerrorRate,
		p.SameSrvRate,
		p.DiffSrvRate,
		p.SrvDiffHostRate,
		p.DstHostCount,
		p.DstHostSrvCount,
		p.DstHostSameSrvRate,
		p.DstHostDiffSrvRate,
		p.DstHostSameSrcPortRate,
		p.DstHostSrvDiffHostRate,
		p.DstHostSerrorRate,
		p.DstHostSrvSerrorRate,
		p.DstHostRerrorRate,
		p.DstHostSrvRerrorRate,
	}

	return inputVector
}
