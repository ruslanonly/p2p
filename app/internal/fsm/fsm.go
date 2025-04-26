package fsm

import (
	"context"
	"log"

	"github.com/looplab/fsm"
)

type AgentFSM struct {
	FSM *fsm.FSM
	ctx context.Context
}

func NewAgentFSM(ctx context.Context, callbacks fsm.Callbacks) *AgentFSM {
	FSM := fsm.NewFSM(
		IdleAgentFSMState,
		NodeFSMEvents,
		callbacks,
	)

	agentFSM := &AgentFSM{
		FSM: FSM,
		ctx: ctx,
	}

	return agentFSM
}

func (afsm *AgentFSM) Event(event string, args ...interface{}) error {
	err := afsm.FSM.Event(afsm.ctx, event, args...)
	if (err != nil) {
		log.Printf("Ошибка при FSM переходе: %v", err)
		return err
	}

	return nil
}