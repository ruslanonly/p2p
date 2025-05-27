package ip

import "net"

func MyIP() net.IP {
	interfaces, err := net.Interfaces()
	if err != nil {
		return net.IPv4zero
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
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

			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			return ip
		}
	}

	return net.IPv4zero
}

func MyIPs() []net.IP {
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

func IsLocalIP(ip net.IP) bool {
	localIPs := MyIPs()

	for _, localIP := range localIPs {
		if localIP.Equal(ip) {
			return true
		}
	}
	return false
}
