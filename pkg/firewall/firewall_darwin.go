//go:build darwin

package firewall

import (
	"bytes"
	"net"
	"os/exec"
	"strings"
)

type darwinFirewall struct{}

func newFirewall() FirewallManager {
	return &darwinFirewall{}
}

func (f *darwinFirewall) Block(ip net.IP) error {
	cmd := exec.Command("sudo", "pfctl", "-t", "blocked_ips", "-T", "add", ip.String())
	return cmd.Run()
}

func (f *darwinFirewall) Unblock(ip net.IP) error {
	cmd := exec.Command("sudo", "pfctl", "-t", "blocked_ips", "-T", "delete", ip.String())
	return cmd.Run()
}

func (f *darwinFirewall) BlockedList() []net.IP {
	cmd := exec.Command("sudo", "pfctl", "-t", "blocked_ips", "-T", "show")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var ips []net.IP
	lines := bytes.Split(output, []byte("\n"))
	for _, line := range lines {
		ipStr := strings.TrimSpace(string(line))
		if ipStr == "" {
			continue
		}
		ip := net.ParseIP(ipStr)
		if ip != nil {
			ips = append(ips, ip)
		}
	}
	return ips
}
