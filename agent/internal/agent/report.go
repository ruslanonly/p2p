package agent

import (
	"net"
	"pkg/report"
)

func (a *Agent) Report(name string) {
	var neighbours []report.AgentReportNeighbor = make([]report.AgentReportNeighbor, 0)
	for _, peer := range a.peers {
		neighbours = append(neighbours, report.AgentReportNeighbor{
			ID:    peer.ID.String(),
			IsHub: !peer.Status.IsAbonent(),
		})
	}

	r := report.NewAgentReport(
		a.node.Host.ID().String(),
		name,
		[]net.IP{},
		neighbours,
	)

	report.Report(r)
}
