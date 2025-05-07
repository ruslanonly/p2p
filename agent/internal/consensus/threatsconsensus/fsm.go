package threatsconsensus

import (
	"fmt"
	"io"

	"github.com/hashicorp/raft"
)

type ThreatsConsensusRaftFSM struct {
	currentHub string
}

func (h *ThreatsConsensusRaftFSM) Apply(logEntry *raft.Log) interface{} {
	h.currentHub = string(logEntry.Data)
	fmt.Println("✅ Новый выбранный хаб:", h.currentHub)
	return nil
}

func (h *ThreatsConsensusRaftFSM) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}

func (h *ThreatsConsensusRaftFSM) Restore(rc io.ReadCloser) error {
	return nil
}
