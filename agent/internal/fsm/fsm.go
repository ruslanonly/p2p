package fsm

import (
	"context"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/looplab/fsm"
	statusmodel "github.com/ruslanonly/agent/internal/agent/model/status"
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
	if err != nil {
		log.Printf("Ошибка при FSM переходе: %v", err)
		return err
	}

	return nil
}

func (afsm *AgentFSM) metadata(key AgentFSMMetadataKey) (interface{}, bool) {
	return afsm.FSM.Metadata(string(key))
}

func (afsm *AgentFSM) setMetadata(key AgentFSMMetadataKey, dataValue interface{}) {
	afsm.FSM.SetMetadata(string(key), dataValue)
}

func (afsm *AgentFSM) deleteMetadata(key AgentFSMMetadataKey) {
	afsm.FSM.DeleteMetadata(string(key))
}

func (afsm *AgentFSM) IsHub() (bool, error) {
	isHubRaw, found := afsm.metadata(_IsHubAgentFSMMetadataKey)
	if found {
		isHub, asserted := isHubRaw.(bool)
		if asserted {
			return isHub, nil
		} else {
			afsm.deleteMetadata(_IsHubAgentFSMMetadataKey)
			return false, fmt.Errorf("ошибка при определении статуса")
		}
	}

	return false, fmt.Errorf("ошибка при определении статуса")
}

func (afsm *AgentFSM) IAmHub() {
	afsm.setMetadata(_IsHubAgentFSMMetadataKey, true)
}

func (afsm *AgentFSM) IAmAbonent() {
	afsm.setMetadata(_IsHubAgentFSMMetadataKey, false)
}

func (afsm *AgentFSM) GetPendingHubPeers() []peer.AddrInfo {
	setRaw, found := afsm.metadata(_PendingHubPeersAgentFSMMetadataKey)
	if !found {
		return nil
	}

	pendingHubPeers := setRaw.(map[peer.ID]peer.AddrInfo)
	peers := make([]peer.AddrInfo, 0, len(pendingHubPeers))
	for _, addr := range pendingHubPeers {
		peers = append(peers, addr)
	}
	return peers
}

func (afsm *AgentFSM) AddPendingHubPeer(addrInfo peer.AddrInfo) {
	setRaw, found := afsm.metadata(_PendingHubPeersAgentFSMMetadataKey)
	var pendingHubPeers map[peer.ID]peer.AddrInfo

	if found {
		pendingHubPeers, _ = setRaw.(map[peer.ID]peer.AddrInfo)
	} else {
		pendingHubPeers = make(map[peer.ID]peer.AddrInfo)
	}

	pendingHubPeers[addrInfo.ID] = addrInfo
	afsm.setMetadata(_PendingHubPeersAgentFSMMetadataKey, pendingHubPeers)
}

func (afsm *AgentFSM) RemovePendingHubPeer(peerID peer.ID) {
	setRaw, found := afsm.metadata(_PendingHubPeersAgentFSMMetadataKey)
	if found {
		pendingHubPeers := setRaw.(map[peer.ID]peer.AddrInfo)
		delete(pendingHubPeers, peerID)
		afsm.setMetadata(_PendingHubPeersAgentFSMMetadataKey, pendingHubPeers)
	}
}

type KnownHub struct {
	ID     peer.ID
	Addrs  []string
	Status statusmodel.PeerP2PStatus
}

func (afsm *AgentFSM) GetKnownHubs() []KnownHub {
	setRaw, found := afsm.metadata(_KnownHubsAgentFSMMetadataKey)
	if !found {
		return nil
	}

	knownHubs := setRaw.(map[peer.ID]KnownHub)
	peers := make([]KnownHub, 0, len(knownHubs))
	for _, addr := range knownHubs {
		peers = append(peers, addr)
	}

	return peers
}

func (afsm *AgentFSM) AddKnownHub(hub KnownHub) {
	setRaw, found := afsm.metadata(_KnownHubsAgentFSMMetadataKey)
	var knownHubs map[peer.ID]KnownHub

	if found {
		knownHubs, _ = setRaw.(map[peer.ID]KnownHub)
	} else {
		knownHubs = make(map[peer.ID]KnownHub)
	}

	knownHubs[hub.ID] = hub
	afsm.setMetadata(_KnownHubsAgentFSMMetadataKey, knownHubs)
}

func (afsm *AgentFSM) RemoveKnownHub(peerID peer.ID) {
	setRaw, found := afsm.metadata(_KnownHubsAgentFSMMetadataKey)
	if found {
		knownHubs := setRaw.(map[peer.ID]KnownHub)
		delete(knownHubs, peerID)
		afsm.setMetadata(_KnownHubsAgentFSMMetadataKey, knownHubs)
	}
}

func (afsm *AgentFSM) GetElectionPeers() []peer.ID {
	raw, found := afsm.metadata(_ElectionPeerIDsAgentFSMMetadataKey)
	if !found {
		return nil
	}

	knownHubs, ok := raw.([]peer.ID)
	if !ok {
		return make([]peer.ID, 0)
	}

	return knownHubs
}

func (afsm *AgentFSM) SetElectionPeers(peers []peer.ID) {
	var electionPeers []peer.ID = peers

	afsm.setMetadata(_ElectionPeerIDsAgentFSMMetadataKey, electionPeers)
}

func (afsm *AgentFSM) DeleteElectionPeers() {
	afsm.deleteMetadata(_ElectionPeerIDsAgentFSMMetadataKey)
}
