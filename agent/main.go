package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	agentPkg "github.com/ruslanonly/agent/internal/agent"
)

func main() {
	port := 5000

	agentID := os.Getenv("AGENT_ID")
	if agentID == "" {
		log.Fatalf("Параметр AGENT_ID не указан")
	}

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

	go (func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				agent.Report(fmt.Sprintf("AGENT %s", agentID))
			}
		}
	})()

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
