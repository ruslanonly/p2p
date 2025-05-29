package report

import (
	"net"

	"github.com/libp2p/go-libp2p/core/peer"
)

type AgentReportNeighbour struct {
	ID    string `json:"id"`
	IsHub bool   `json:"is_hub"`
}

type AgentReport struct {
	ID            string                 `json:"agent_id"`
	Name          string                 `json:"name"`
	State         string                 `json:"state"`
	Neighbors     []AgentReportNeighbour `json:"neighbors"`
	YellowReports map[string][]peer.ID   `json:"yellow_reports"`
	RedReports    map[string][]peer.ID   `json:"red_reports"`
	BlockedHosts  []net.IP               `json:"blocked_reports"`
}

func NewAgentReport(
	id, name, state string,
	neighbors []AgentReportNeighbour,
	yellowReports,
	redReports map[string][]peer.ID,
	blocked []net.IP,
) AgentReport {
	return AgentReport{
		ID:            id,
		Name:          name,
		State:         state,
		Neighbors:     neighbors,
		YellowReports: yellowReports,
		RedReports:    redReports,
		BlockedHosts:  blocked,
	}
}
