package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

const ProtocolID = "/protocol/1.0.0"

func main() {
	ctx := context.Background()

	host, err := libp2p.New()
	if err != nil {
		log.Fatalf("Ошибка при создании хоста: %v", err)
	}

	host.SetStreamHandler(ProtocolID, func(s network.Stream) {
		defer s.Close()
		r := bufio.NewReader(s)
		msg, _ := r.ReadString('\n')
		fmt.Printf("📥 Получено сообщение: %s", msg)
	})

	time.Sleep(3 * time.Second)

	targetAddrStr := "/ip4/127.0.0.1/tcp/5001/p2p/12D3KooWNDq8CpTzRdsQp8KmZrp3qVnGXkBAY4AXs1sNJqBYmYmx"
	targetAddr, err := ma.NewMultiaddr(targetAddrStr)
	if err != nil {
		log.Fatalf("Невозможно разобрать multiaddr: %v", err)
	}

	info, err := peer.AddrInfoFromP2pAddr(targetAddr)
	if err != nil {
		log.Fatalf("Невозможно извлечь peer info: %v", err)
	}

	if err := host.Connect(ctx, *info); err != nil {
		log.Fatalf("❌ Ошибка при подключении к пиру: %v", err)
	}

	stream, err := host.NewStream(ctx, info.ID, ProtocolID)
	if err != nil {
		log.Fatalf("❌ Ошибка при создании стрима: %v", err)
	}
	defer stream.Close()

	_, err = stream.Write([]byte("ping\n"))
	if err != nil {
		log.Fatalf("❌ Ошибка при отправке сообщения: %v", err)
	}

	log.Println("✅ Сообщение отправлено")
}
