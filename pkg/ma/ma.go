package ma

import (
	"net"

	multiaddr "github.com/multiformats/go-multiaddr"
)

func MultiaddrToIP(addr multiaddr.Multiaddr) net.IP {
	ipComp, err := addr.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		ipComp, err = addr.ValueForProtocol(multiaddr.P_IP6)
		if err != nil {
			return nil
		}
	}
	return net.ParseIP(ipComp)
}
