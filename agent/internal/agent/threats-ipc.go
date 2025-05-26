package agent

import (
	"log"
	"pkg/threats/model"
)

func (a *Agent) RedTrafficIPCHandler(body model.RedTrafficMessageTypeBody) {
	log.Printf("🎩 [IPC] Красный трафик %s", body.IP)

	a.threatsStorage.BlockHost(body.IP, "self blocked")
	if isHub, err := a.fsm.IsHub(); err == nil {
		if isHub {
			a.RedTrafficHubMessage(body.IP)
			a.broadcastBlockTrafficToAbonents(body.IP)
		} else {
			a.informMyHubAboutRedTraffic(body.IP)
		}
	}
}

func (a *Agent) YellowTrafficIPCHandler(body model.YellowTrafficMessageTypeBody) {
	log.Printf("🎩 [IPC] Подозрительный трафик %s", body.IP)
	a.threatsStorage.ReportYellowThreat(body.IP, a.node.Host.ID())

	if isHub, err := a.fsm.IsHub(); err == nil {
		if isHub {
			a.YellowTrafficHubMessage(body.IP)
		} else {
			a.informMyHubAboutYellowTraffic(body.IP)
		}
	}
}
