package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

const ProtocolID = "/protocol/1.0.0"

// Number of concurrent streams to open
const concurrency = 50

// Number of messages per stream
const messagesPerStream = 100

func main() {
	ctx := context.Background()

	// Create libp2p host
	host, err := libp2p.New()
	if err != nil {
		log.Fatalf("Failed to create host: %v", err)
	}

	// Set stream handler to print incoming messages
	host.SetStreamHandler(ProtocolID, func(s network.Stream) {
		defer s.Close()
		r := bufio.NewReader(s)
		for {
			msg, err := r.ReadString('\n')
			if err != nil {
				return
			}
			fmt.Printf("📥 Received message: %s", msg)
		}
	})

	time.Sleep(3 * time.Second)

	targetAddrStr := "/ip4/127.0.0.1/tcp/5010/p2p/12D3KooWA5BqSnQpdpcyqVWVQEixL39yaxNnWud8ZXrYf3TdpkJm"
	var info *peer.AddrInfo
	var connectErr error
	const retryInterval = 3 * time.Second

	for {
		maddr, err := ma.NewMultiaddr(targetAddrStr)
		if err != nil {
			log.Fatalf("Failed to parse multiaddr: %v", err)
		}

		info, err = peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Printf("⚠️ Failed to get peer info: %v", err)
			time.Sleep(retryInterval)
			continue
		}

		connectErr = host.Connect(ctx, *info)
		if connectErr == nil {
			break // success
		}

		log.Printf("🚫 Failed to connect to peer: %v", connectErr)

		if strings.Contains(connectErr.Error(), "peer id mismatch") {
			re := regexp.MustCompile(`remote key matches ([\w\d]+)`)
			matches := re.FindStringSubmatch(connectErr.Error())
			if len(matches) > 1 {
				actualPeerID := matches[1]
				log.Printf("⚠️ Detected actual PeerID: %s", actualPeerID)
				targetAddrStr = updatePeerID(targetAddrStr, actualPeerID)
				log.Printf("🔁 Retrying with updated multiaddr: %s", targetAddrStr)
			}
		}

		time.Sleep(retryInterval)
	}

	log.Println("✅ Connected to target peer")

	var wg sync.WaitGroup

	// Launch multiple goroutines to open streams concurrently
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			stream, err := host.NewStream(ctx, info.ID, ProtocolID)
			if err != nil {
				log.Printf("Stream %d: failed to create stream: %v", id, err)
				return
			}
			defer stream.Close()

			for j := 0; j < messagesPerStream; j++ {
				msg := fmt.Sprintf("ping from stream %d message %d\n", id, j)
				_, err := stream.Write([]byte(msg))
				if err != nil {
					log.Printf("Stream %d: failed to send message: %v", id, err)
					return
				}
				time.Sleep(10 * time.Millisecond)
			}

			log.Printf("Stream %d: finished sending messages", id)
		}(i)
	}

	wg.Wait()
	log.Println("✅ All streams finished")
}

// updatePeerID заменяет старый PeerID в multiaddr на новый
func updatePeerID(multiaddrStr, newPeerID string) string {
	idx := strings.LastIndex(multiaddrStr, "/p2p/")
	if idx == -1 {
		return multiaddrStr
	}
	return multiaddrStr[:idx] + "/p2p/" + newPeerID
}
