package model

import (
	"github.com/libp2p/go-libp2p/core/peer"
	statusmodel "github.com/ruslanonly/agent/internal/agent/model/status"
)

type AgentPeerInfoPeer struct {
	ID     peer.ID
	Addrs  []string
	Status statusmodel.PeerP2PStatus
}

type AgentPeerInfo struct {
	ID     peer.ID
	Status statusmodel.PeerP2PStatus
	Peers  map[peer.ID]AgentPeerInfoPeer
}
