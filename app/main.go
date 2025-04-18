package main

import (
	"log"
	"os"
	"strconv"

	agent "github.com/ruslanonly/p2p/host"
	"github.com/ruslanonly/p2p/lib"
)

func main() {
	host, err := lib.LocalIP()
	if (err != nil) {
		log.Fatalf("Не удалось получить IP адрес: %v", err)
	}

	maxPeers, err := strconv.Atoi(os.Getenv("MAX_PEERS"))
	if (err != nil) {
		log.Fatalf("Параметр MAX_PEERS указан неверно: %v", err)
	} else if (maxPeers <= 1) {
		log.Fatalf("Параметр MAX_PEERS указан неверно: максимальное количество пиров у узла не может быть меньше 1")
	}

	a := agent.New(agent.AgentConfiguration{
		Host: host,
		MaxPeers: maxPeers,
	})

	bootstrapIP := os.Getenv("BOOTSTRAP_IP")

	if (err != nil) {
		log.Fatalf("Параметр BOOTSTRAP_PORT указан неверно: %v", err)
	}

	if (bootstrapIP == "") {
		a.Start(nil)
	} else {
		a.Start(&agent.BootstrapAddress{
			IP: bootstrapIP,
		})
	}
}
