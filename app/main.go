package main

import (
	"context"
	"log"
	"os"

	agentPkg "github.com/ruslanonly/p2p/internal/agent"
)

func main() {
	port := 5000

	agent, err := agentPkg.NewAgent(context.Background(), 2, port)
	if (err != nil) {
		log.Fatalf("Ошибка при инициализации узла: %v", err)
	}

	bootstrapIP := os.Getenv("BOOTSTRAP_IP")
	if (err != nil) {
		log.Fatalf("Параметр BOOTSTRAP_PORT указан неверно: %v", err)
	}

	bootstrapPeerID := os.Getenv("BOOTSTRAP_PEER_ID")
	if (err != nil) {
		log.Fatalf("Параметр BOOTSTRAP_PEER_ID указан неверно: %v", err)
	}

	if (bootstrapIP != "" && bootstrapPeerID != "") {
		log.Printf("Bootstrap IP: %s", bootstrapIP)

		agent.Start(&agentPkg.StartOptions{
			BootstrapIP: bootstrapIP,
			BootstrapPeerID: bootstrapPeerID,
		});
	} else {
		agent.Start(nil);
	}
}