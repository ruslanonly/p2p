package sniffer

import "net"

func getLocalIPs() []net.IP {
	var ips []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // интерфейс выключен
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip.To4() != nil {
				ips = append(ips, ip)
			}
		}
	}
	return ips
}

func isLocalIP(ip net.IP) bool {
	localIPs := getLocalIPs()

	for _, localIP := range localIPs {
		if localIP.Equal(ip) {
			return true
		}
	}
	return false
}
