package status

type PeerP2PStatus string

const (
	AbonentP2PStatus               PeerP2PStatus = "AbonentP2PStatus"
	HubFullP2PStatus               PeerP2PStatus = "HubFullP2PStatus"
	HubFullHavingAbonentsP2PStatus PeerP2PStatus = "HubFullHavingAbonentsP2PStatus"
	HubFreeP2PStatus               PeerP2PStatus = "HubFreeP2PStatus"
)

func (p PeerP2PStatus) IsAbonent() bool {
	return p == AbonentP2PStatus
}

func (p PeerP2PStatus) IsHub() bool {
	return p != AbonentP2PStatus
}

func (p PeerP2PStatus) ToHubSlotsStatus() HubSlotsStatus {
	switch p {
	case HubFreeP2PStatus:
		return FreeHubSlotsStatus
	case HubFullHavingAbonentsP2PStatus:
		return FullHavingAbonentsHubSlotsStatus
	case HubFullP2PStatus:
	}

	return FullNotHavingAbonentsHubSlotsStatus
}
