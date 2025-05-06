package sniffer

import (
	"fmt"

	"github.com/google/gopacket/pcap"
)

func GetDefaultInterface() (string, error) {
	ifaces, err := pcap.FindAllDevs()
	if err != nil {
		return "", fmt.Errorf("ошибка получения интерфейсов: %w", err)
	}

	for _, iface := range ifaces {
		for _, addr := range iface.Addresses {
			fmt.Println("Адреса: ", addr.IP.String())
			if addr.IP.To4() != nil {
				return iface.Name, nil
			}
		}
	}
	return "", fmt.Errorf("не найден подходящий интерфейс")
}
