package network

import (
	"fmt"

	"github.com/multiformats/go-multiaddr"
)

func MultiaddrsStrsToMultiaddrs(multiaddrStrs []string) ([]multiaddr.Multiaddr, error) {
	var mas []multiaddr.Multiaddr

	for _, multiaddrStr := range multiaddrStrs {
		ma, err := multiaddr.NewMultiaddr(multiaddrStr)
		if err != nil {
			return nil, fmt.Errorf("ошибка конвертации multiaddr в AddrInfo %s: %w", multiaddrStr, err)
		}
		mas = append(mas, ma)
	}

	return mas, nil
}

func MultiaddrsToMultiaddrStrs(mas []multiaddr.Multiaddr) []string {
	var masStrs []string

	for _, ma := range mas {
		masStrs = append(masStrs, ma.String())
	}

	return masStrs
}
