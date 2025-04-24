package fsm

import (
	"github.com/looplab/fsm"
)

type AgentFSM struct {
	FSM *fsm.FSM
}

func NewAgentFSM(callbacks fsm.Callbacks) *AgentFSM {
	FSM := fsm.NewFSM(
		IdleAgentFSMState,
		NodeFSMEvents,
		callbacks,
	)

	agentFSM := &AgentFSM{
		FSM: FSM,
	}

	return agentFSM
}