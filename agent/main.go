package main

import (
	"context"
	"log"
	"os"

	agentPkg "github.com/ruslanonly/agent/internal/agent"
)

func main() {
	port := 5000

	maxPeers := os.Getenv("PEERS_LIMIT")
	if maxPeers == "" {
		log.Fatalf("Параметр PEERS_LIMIT не указан")
	}

	agent, err := agentPkg.NewAgent(context.Background(), 3, port)
	if err != nil {
		log.Fatalf("Ошибка при инициализации узла: %v", err)
	}

	bootstrapIP := os.Getenv("BOOTSTRAP_IP")
	bootstrapPeerID := os.Getenv("BOOTSTRAP_PEER_ID")

	if bootstrapIP != "" && bootstrapPeerID != "" {
		log.Printf("Bootstrap IP: %s", bootstrapIP)

		agent.Start(&agentPkg.StartOptions{
			BootstrapIP:     bootstrapIP,
			BootstrapPeerID: bootstrapPeerID,
		})
	} else {
		agent.Start(nil)
	}
}
