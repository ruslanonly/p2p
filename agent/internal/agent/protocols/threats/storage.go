package threats

import (
	"log"
	"net"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

type ThreatsStorage struct {
	mu            sync.Mutex
	YellowReports map[string][]peer.ID
	RedReports    map[string][]peer.ID
	BlockedHosts  map[string]bool

	onBlock func(blockedIP net.IP)
}

// NewThreatsStorage создаёт новый экземпляр ThreatsStorage
func NewThreatsStorage(onBlock func(blockedIP net.IP)) *ThreatsStorage {
	return &ThreatsStorage{
		YellowReports: make(map[string][]peer.ID),
		RedReports:    make(map[string][]peer.ID),
		BlockedHosts:  make(map[string]bool),
		onBlock:       onBlock,
	}
}

// ReportYellowThreat добавляет жёлтый репорт от указанного пира
func (ts *ThreatsStorage) ReportYellowThreat(targetIP net.IP, reporter peer.ID) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ipStr := targetIP.String()
	if ts.BlockedHosts[ipStr] {
		return
	}

	if !containsPeer(ts.YellowReports[ipStr], reporter) {
		ts.YellowReports[ipStr] = append(ts.YellowReports[ipStr], reporter)
	}

	if len(ts.YellowReports[ipStr]) >= 3 {
		ts.BlockHost(targetIP, "yellow threshold reached")
	}
}

// ReportRedThreat добавляет красный репорт от указанного пира
func (ts *ThreatsStorage) ReportRedThreat(targetIP net.IP, reporter peer.ID) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ipStr := targetIP.String()
	if ts.BlockedHosts[ipStr] {
		return
	}

	if !containsPeer(ts.RedReports[ipStr], reporter) {
		ts.RedReports[ipStr] = append(ts.RedReports[ipStr], reporter)
	}

	if len(ts.RedReports[ipStr]) >= 3 {
		ts.BlockHost(targetIP, "red threshold reached")
	}
}

// IsBlocked проверяет, заблокирован ли указанный IP
func (ts *ThreatsStorage) IsBlocked(ip net.IP) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.BlockedHosts[ip.String()]
}

// Внутренняя функция блокировки
func (ts *ThreatsStorage) BlockHost(ip net.IP, reason string) {
	ipStr := ip.String()
	ts.BlockedHosts[ipStr] = true
	delete(ts.YellowReports, ipStr)
	delete(ts.RedReports, ipStr)

	log.Printf("🚫 Заблокирован IP %s: %s", ipStr, reason)

	if ts.onBlock != nil {
		go ts.onBlock(ip)
	}
}

// Вспомогательная функция: проверяет, содержится ли peer.ID в списке
func containsPeer(peers []peer.ID, id peer.ID) bool {
	for _, p := range peers {
		if p == id {
			return true
		}
	}
	return false
}
