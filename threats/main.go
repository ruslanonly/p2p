package main

import (
	"log"
	"net"
	"pkg/firewall"
	"pkg/threats"
	threatsModel "pkg/threats/model"
	"threats/internal/classifier"
	classifierModel "threats/internal/classifier/model"
	"threats/internal/sniffer"

	"github.com/google/gopacket"
)

func main() {
	iface, err := sniffer.GetDefaultInterface()
	if err != nil {
		log.Fatalf("❌ Не удалось определить интерфейс: %v", err)
	}

	snf, err := sniffer.NewSniffer(iface)
	if err != nil {
		log.Fatalf("❌ Не удалось запустить сниффер: %v", err)
	}

	ipc, err := threats.NewThreatsIPCServer()
	if err != nil {
		log.Fatalf("❌ Не удалось инициализировать IPC-сервер: %v", err)
	}

	var fw firewall.FirewallManager = firewall.New()

	fw.Block(net.IPv4(byte(172), byte(18), byte(0), byte(2)))

	cls := classifier.NewClassifier()
	snf.Run(func(packet gopacket.Packet) {
		parameters, err1 := classifier.ExtractTCPIPParameters(packet)

		if err1 != nil {
			return
		}

		trafficClass := cls.Classify(*parameters)

		if trafficClass == classifierModel.GreenTrafficClass {

		} else if trafficClass == classifierModel.YellowTrafficClass {
			ipc.YellowTrafficMessage(threatsModel.YellowTrafficMessageTypeBody{
				IP: parameters.SrcIP.String(),
			})
		} else {
			ipc.RedTrafficMessage(threatsModel.RedTrafficMessageTypeBody{
				IP: parameters.SrcIP.String(),
			})

			fw.Block(parameters.SrcIP)
		}
	})
}
