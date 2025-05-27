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

	cls := classifier.NewClassifier()

	go snf.Run(func(parameters *classifierModel.TrafficParameters) {
		if fw.IsBlocked(parameters.SrcIP) {
			return
		}

		vector := parameters.Vectorize()
		trafficClass := cls.Classify(vector)

		// trafficClass = classifierModel.GreenTrafficClass
		// if parameters.SrcIP.Equal(net.ParseIP("192.168.65.1")) {
		// 	trafficClass = classifierModel.RedTrafficClass
		// }

		if trafficClass == classifierModel.YellowTrafficClass {
			log.Printf("🟡 [FIREWALL] Подозрительный узел %s", parameters.SrcIP)
			ipc.YellowTrafficMessage(threatsModel.YellowTrafficMessageTypeBody{
				IP: parameters.SrcIP,
			})
		} else if trafficClass == classifierModel.RedTrafficClass {
			log.Printf("🛑 [FIREWALL] Обнаружен красный узел %s %v", parameters.SrcIP, vector)
			log.Printf("🛑 [FIREWALL] Блокировка узла %s", parameters.SrcIP)

			fw.Block(parameters.SrcIP)

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
