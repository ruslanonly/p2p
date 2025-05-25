package firewall

import "net"

type FirewallManager interface {
	IsBlocked(ip net.IP) bool
	Block(ip net.IP) error
	Unblock(ip net.IP) error
	BlockedList() []net.IP
}

func New() FirewallManager {
	return newFirewall()
}
