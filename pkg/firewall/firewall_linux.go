//go:build linux

package firewall

import (
	"net"
	"os/exec"
)

type linuxFirewall struct{}

func newFirewall() FirewallManager {
	return &linuxFirewall{}
}

func (f *linuxFirewall) Block(ip net.IP) error {
	return exec.Command("iptables", "-A", "INPUT", "-s", ip.String(), "-j", "DROP").Run()
}

func (f *linuxFirewall) Unblock(ip net.IP) error {
	return exec.Command("iptables", "-D", "INPUT", "-s", ip.String(), "-j", "DROP").Run()
}
