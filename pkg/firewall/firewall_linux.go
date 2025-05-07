//go:build linux

package firewall

import (
	"bufio"
	"bytes"
	"net"
	"os/exec"
	"strings"
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

func (f *linuxFirewall) BlockedList() []net.IP {
	cmd := exec.Command("iptables", "-L", "INPUT", "-v", "-n")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var blocked []net.IP
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "DROP") {
			fields := strings.Fields(line)
			if len(fields) >= 8 {
				srcIP := fields[7]
				ip := net.ParseIP(srcIP)
				if ip != nil {
					blocked = append(blocked, ip)
				}
			}
		}
	}

	return blocked
}
