package firewall

import "net"

type FirewallManager interface {
	Block(ip net.IP) error
	Unblock(ip net.IP) error
	BlockedList() []net.IP
}

func New() FirewallManager {
	return newFirewall()
}
