package agent

import (
	"log"
	"pkg/threats/model"
)

func (a *Agent) RedTrafficIPCHandler(body model.RedTrafficMessageTypeBody) {
	log.Printf("🎩 [IPC] Красный трафик %s", body.IP)
	a.threatsIPC.BlockHostMessage(body.IP)

	if isHub, err := a.fsm.IsHub(); err == nil {
		if isHub {
			a.RedTrafficHubMessage(body.IP)
		} else {
			a.informMyHubAboutRedTraffic(body.IP)
		}
	}
}

func (a *Agent) YellowTrafficIPCHandler(body model.YellowTrafficMessageTypeBody) {
	log.Printf("🎩 [IPC] Подозрительный трафик %s", body.IP)
}
