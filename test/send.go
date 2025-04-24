package main

import (
	"context"
	"log"

	"github.com/libp2p/go-libp2p"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

func main() {
	ctx := context.Background()

	h, err := libp2p.New()
	if err != nil {
		log.Fatal(err)
	}

	peerAddr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/6970/p2p/12D3KooWRFAzyWMGHDU3Z3fK9WwLecQptc4XShuWrcoGsFaK3xtA")

	info, _ := libpeer.AddrInfoFromP2pAddr(peerAddr)
	if err := h.Connect(ctx, *info); err != nil {
		log.Fatal(err)
	}

	s, err := h.NewStream(ctx, info.ID, "/agent/1.0.0")
	if err != nil {
		log.Fatal(err)
	}

	_, err = s.Write([]byte("ping from peer\n"))
	if err != nil {
		log.Fatal(err)
	}

	s.Close()
}
