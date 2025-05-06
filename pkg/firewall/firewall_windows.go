//go:build windows

package firewall

import (
	"net"
	"os/exec"
)

type windowsFirewall struct{}

func newFirewall() FirewallManager {
	return &windowsFirewall{}
}

func (f *windowsFirewall) Block(ip net.IP) error {
	return exec.Command("netsh", "advfirewall", "firewall", "add", "rule", "name=BlockIP", "dir=in", "action=block", "remoteip="+ip.String()).Run()
}

func (f *windowsFirewall) Unblock(ip net.IP) error {
	return exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", "name=BlockIP", "remoteip="+ip.String()).Run()
}
