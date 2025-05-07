//go:build windows

package firewall

import (
	"bufio"
	"bytes"
	"net"
	"os/exec"
	"strings"
)

type windowsFirewall struct{}

func newFirewall() FirewallManager {
	return &windowsFirewall{}
}

func (f *windowsFirewall) Block(ip net.IP) error {
	return exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=BlockIP_"+ip.String(),
		"dir=in", "action=block",
		"remoteip="+ip.String()).Run()
}

func (f *windowsFirewall) Unblock(ip net.IP) error {
	return exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name=BlockIP_"+ip.String(),
		"remoteip="+ip.String()).Run()
}

func (f *windowsFirewall) BlockedList() []net.IP {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name=all")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var blocked []net.IP
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var inBlockRule bool
	for scanner.Scan() {
		line := scanner.Text()

		// Ищем начало правила
		if strings.HasPrefix(line, "Rule Name:") && strings.Contains(line, "BlockIP_") {
			inBlockRule = true
		}

		if inBlockRule && strings.HasPrefix(line, "RemoteIP:") {
			ipStr := strings.TrimSpace(strings.TrimPrefix(line, "RemoteIP:"))
			if ip := net.ParseIP(ipStr); ip != nil {
				blocked = append(blocked, ip)
			}
			inBlockRule = false
		}
	}
	return blocked
}
