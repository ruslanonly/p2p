package agent

import (
	"net"
	"pkg/report"
)

func (a *Agent) Report(name string) {
	var neighbours []report.AgentReportNeighbour = make([]report.AgentReportNeighbour, 0)
	for _, peer := range a.peers {
		neighbours = append(neighbours, report.AgentReportNeighbour{
			ID:    peer.ID.String(),
			IsHub: peer.Status.IsHub(),
		})
	}

	var blocked []net.IP = make([]net.IP, 0)
	for rawIP := range a.threatsStorage.BlockedHosts {
		ip := net.ParseIP(rawIP)
		blocked = append(blocked, ip)
	}

	var peerHubsIDs []string = make([]string, 0)
	var peerAbonentsIDs []string = make([]string, 0)
	for _, peer := range a.peers {
		if peer.Status.IsHub() {
			peerHubsIDs = append(peerHubsIDs, peer.ID.String())
		} else {
			peerAbonentsIDs = append(peerAbonentsIDs, peer.ID.String())
		}
	}

	r := report.NewAgentReport(
		a.node.Host.ID().String(),
		name,
		a.fsm.FSM.Current(),
		neighbours,
		a.threatsStorage.YellowReports,
		a.threatsStorage.RedReports,
		blocked,
		peerHubsIDs,
		peerAbonentsIDs,
	)

	report.Report(r)
}
