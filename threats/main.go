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

	cls := classifier.NewClassifier()

	go snf.Run(func(packet gopacket.Packet) {
		parameters, err1 := classifier.ExtractTCPIPParameters(packet)

		if err1 != nil {
			return
		}

		trafficClass := cls.Classify(*parameters)

		if trafficClass == classifierModel.GreenTrafficClass {

		} else if trafficClass == classifierModel.YellowTrafficClass {
			ipc.YellowTrafficMessage(threatsModel.YellowTrafficMessageTypeBody{
				IP: parameters.SrcIP,
			})
		} else {
			fw.Block(parameters.SrcIP)
			ipc.RedTrafficMessage(threatsModel.RedTrafficMessageTypeBody{
				IP: parameters.SrcIP,
			})
		}
	})

	ipc.Listen(
		func(body threatsModel.BlockHostMessageTypeBody) {
			log.Println("Блокировка FIREWALL")
			fw.Block(net.IP(body.IP))
		},
		func(body threatsModel.MercyHostMessageTypeBody) {
			fw.Unblock(net.IP(body.IP))
		},
	)
}
