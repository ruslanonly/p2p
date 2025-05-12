package status

type HubSlotsStatus string

const (
	// Есть свободные слоты для подключения
	FreeHubSlotsStatus HubSlotsStatus = "FreeHubSlotsStatus"
	// Занят, но есть абоненты для инициализации выборов
	FullHavingAbonentsHubSlotsStatus HubSlotsStatus = "FullHavingAbonentsHubSlotsStatus"
	// Полностью занят
	FullNotHavingAbonentsHubSlotsStatus HubSlotsStatus = "FullNotHavingAbonentsHubSlotsStatus"
)

func (p HubSlotsStatus) ToPeerP2PStatus() PeerP2PStatus {
	switch p {
	case FreeHubSlotsStatus:
		return HubFreeP2PStatus
	case FullHavingAbonentsHubSlotsStatus:
		return HubFullHavingAbonentsP2PStatus
	case FullNotHavingAbonentsHubSlotsStatus:
		return HubFullP2PStatus
	}

	return AbonentP2PStatus
}
