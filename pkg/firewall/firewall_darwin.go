//go:build darwin

package firewall

import (
	"fmt"
	"net"
	"os/exec"
)

type darwinFirewall struct{}

func newFirewall() FirewallManager {
	return &darwinFirewall{}
}

func (f *darwinFirewall) Block(ip net.IP) error {
	rule := fmt.Sprintf("block drop from %s to any", ip)
	return exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | sudo pfctl -f -", rule)).Run()
}

func (f *darwinFirewall) Unblock(ip net.IP) error {
	return fmt.Errorf("unblock для macOS требует кастомной реализации")
}
