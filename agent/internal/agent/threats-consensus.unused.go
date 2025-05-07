package agent

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftnet "github.com/libp2p/go-libp2p-raft"
	"github.com/ruslanonly/agent/internal/consensus/threatsconsensus"
)

func (a *Agent) prepareForHubConsensus() (*raft.Raft, error) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(a.node.Host.ID().String())
	config.Logger = hclog.New(&hclog.LoggerOptions{
		Name:  "raft",
		Level: hclog.Error,
	})

	store := raft.NewInmemStore()
	logStore := raft.NewInmemStore()
	snapshotStore := raft.NewDiscardSnapshotStore()

	transport, err := raftnet.NewLibp2pTransport(a.node.Host, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("возникла ошибка при подготовке transport для выборов нового хаба: %v", err)
	}

	hostRaftNode, err := raft.NewRaft(
		config,
		&threatsconsensus.ThreatsConsensusRaftFSM{},
		logStore,
		store,
		snapshotStore,
		transport,
	)

	if err != nil {
		return nil, fmt.Errorf("возникла ошибка при подготовке hostRaftNode для выборов нового хаба: %v", err)
	}

	return hostRaftNode, nil
}

func (a *Agent) initializeHubConsensus() {
	hostRaftNode, err := a.prepareForHubConsensus()
	if err != nil {
		fmt.Println(err)
		return
	}

	addr := a.node.Host.Addrs()[0]

	cfg := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(a.node.Host.ID().String()),
				Address: raft.ServerAddress(addr.String()),
			},
		},
	}

	hostRaftNode.BootstrapCluster(cfg)
}
