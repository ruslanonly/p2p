package report

import (
	"net"
)

type AgentReportNeighbor struct {
	ID    string `json:"id"`
	IsHub bool   `json:"is_hub"`
}

type AgentReport struct {
	ID        string                `json:"agent_id"`
	Name      string                `json:"name"`
	Blocked   []net.IP              `json:"blocked"`
	Neighbors []AgentReportNeighbor `json:"neighbors"`
}

func NewAgentReport(id, name string, blocked []net.IP, neighbors []AgentReportNeighbor) AgentReport {
	return AgentReport{
		ID:        id,
		Name:      name,
		Blocked:   blocked,
		Neighbors: neighbors,
	}
}
