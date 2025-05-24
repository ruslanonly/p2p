package main

import (
	"fmt"
	"log"
	"net"
	"pkg/firewall"
	"pkg/threats"
	threatsModel "pkg/threats/model"
	"threats/internal/classifier"
	classifierModel "threats/internal/classifier/model"
	"threats/internal/sniffer"
)

func main() {
	log.Println("🚀 [THREATS] Запуск threats...")

	iface, err := sniffer.GetDefaultInterface()
	if err != nil {
		log.Fatalf("❌ Не удалось определить интерфейс: %v", err)
	}

	snf, err := sniffer.NewSniffer(iface)
	if err != nil {
		log.Fatalf("❌ Не удалось запустить сниффер: %v", err)
	}

	pipeName := threats.Pipename()
	ipc, err := threats.NewThreatsIPCServer(pipeName)
	if err != nil {
		log.Fatalf("❌ Не удалось инициализировать IPC-сервер: %v", err)
	}

	var fw firewall.FirewallManager = firewall.New()

	aggregator, err := classifier.NewAggregator()
	if err != nil {
		log.Fatalf("❌ Не удалось инициализировать extractor параметров: %v", err)
	}

	cls := classifier.NewClassifier()

	go snf.Run(func(parameters *classifierModel.TrafficParameters) {
		vector := aggregator.Vectorize(*parameters)
		trafficClass := cls.Classify(vector)

		fmt.Println("HANDLER ", trafficClass)
		if trafficClass == classifierModel.GreenTrafficClass {

		} else if trafficClass == classifierModel.YellowTrafficClass {
			ipc.YellowTrafficMessage(threatsModel.YellowTrafficMessageTypeBody{
				IP: parameters.SrcIP,
			})
		} else {
			fw.Block(parameters.SrcIP)
			log.Printf("🛑 [FIREWALL] Блокировка узла %s", parameters.SrcIP)

			ipc.RedTrafficMessage(threatsModel.RedTrafficMessageTypeBody{
				IP: parameters.SrcIP,
			})
		}
	})

	ipc.Listen(
		func(body threatsModel.BlockHostMessageTypeBody) {
			fw.Block(net.IP(body.IP))
			log.Printf("🛑 [FIREWALL] Блокировка узла %s", body.IP)
		},
		func(body threatsModel.MercyHostMessageTypeBody) {
			fw.Unblock(net.IP(body.IP))
			log.Printf("🛑 [FIREWALL] Разблокировка узла %s", body.IP)
		},
	)
}
