package firewall

import "net"

type FirewallManager interface {
	Block(ip net.IP) error
	Unblock(ip net.IP) error
}

func New() FirewallManager {
	return newFirewall()
}
