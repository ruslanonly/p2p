package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
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

	// Target peer multiaddress
	targetAddrStr := "/ip4/127.0.0.1/tcp/5001/p2p/12D3KooWA5BqSnQpdpcyqVWVQEixL39yaxNnWud8ZXrYf3TdpkJm"
	targetAddr, err := ma.NewMultiaddr(targetAddrStr)
	if err != nil {
		log.Fatalf("Failed to parse multiaddr: %v", err)
	}

	info, err := peer.AddrInfoFromP2pAddr(targetAddr)
	if err != nil {
		log.Fatalf("Failed to get peer info: %v", err)
	}

	if err := host.Connect(ctx, *info); err != nil {
		log.Fatalf("Failed to connect to peer: %v", err)
	}

	log.Println("Connected to target peer")

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
				// Optional small delay to avoid overwhelming local resources too fast
				time.Sleep(10 * time.Millisecond)
			}

			log.Printf("Stream %d: finished sending messages", id)
		}(i)
	}

	wg.Wait()
	log.Println("All streams finished")
}
