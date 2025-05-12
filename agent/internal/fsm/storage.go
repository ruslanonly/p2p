package fsm

type AgentFSMMetadataKey string

const (
	_KnownHubsAgentFSMMetadataKey       AgentFSMMetadataKey = "KnownHubsAgentFSMMetadataKey"
	_ElectionPeerIDsAgentFSMMetadataKey AgentFSMMetadataKey = "ElectionPeerIDsAgentFSMMetadataKey"
	_PendingHubPeersAgentFSMMetadataKey AgentFSMMetadataKey = "PendingHubPeersAgentFSMMetadataKey"
	_IsHubAgentFSMMetadataKey           AgentFSMMetadataKey = "IsHubAgentFSMMetadataKey"
)
