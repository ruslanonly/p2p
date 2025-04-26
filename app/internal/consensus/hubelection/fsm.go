package hubelection

import (
	"fmt"
	"io"

	"github.com/hashicorp/raft"
)

type HubElectionRaftFSM struct {
	currentHub string
}

func (h *HubElectionRaftFSM) Apply(logEntry *raft.Log) interface{} {
	h.currentHub = string(logEntry.Data)
	fmt.Println("✅ Новый выбранный хаб:", h.currentHub)
	return nil
}

func (h *HubElectionRaftFSM) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}

func (h *HubElectionRaftFSM) Restore(rc io.ReadCloser) error {
	return nil
}