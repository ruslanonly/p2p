package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

const ProtocolID = "/agent/1.0.0"

type Libp2pAgent struct {
	Host      host.Host
	ctx       context.Context
	cancel    context.CancelFunc
	bootstrap multiaddr.Multiaddr
}

func NewLibp2pAgent(bootstrapAddr string) (*Libp2pAgent, error) {
	ctx, cancel := context.WithCancel(context.Background())

	h, err := libp2p.New(libp2p.ListenAddrStrings(
		"/ip4/0.0.0.0/tcp/0",
	))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %v", err)
	}

	agent := &Libp2pAgent{
		Host:   h,
		ctx:    ctx,
		cancel: cancel,
	}

	h.SetStreamHandler(ProtocolID, agent.handleStream)

	// Parse bootstrap address
	maddr, err := multiaddr.NewMultiaddr(bootstrapAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid bootstrap multiaddr: %v", err)
	}
	agent.bootstrap = maddr

	return agent, nil
}

func (a *Libp2pAgent) Start() {
	log.Printf("🔌 Peer ID: %s", a.Host.ID().String())
	for _, addr := range a.Host.Addrs() {
		log.Printf("🌐 Listening on: %s/p2p/%s", addr, a.Host.ID().String())
	}

	go a.bootstrapConnect()
}

func (a *Libp2pAgent) bootstrapConnect() {
	peerInfo, err := peer.AddrInfoFromP2pAddr(a.bootstrap)
	if err != nil {
		log.Printf("⚠️ Ошибка при разборе bootstrap-адреса: %v", err)
		return
	}

	a.Host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)

	stream, err := a.Host.NewStream(a.ctx, peerInfo.ID, ProtocolID)
	if err != nil {
		log.Printf("❌ Ошибка при подключении к суперпиру: %v", err)
		return
	}
	defer stream.Close()

	msg := Message{Type: "ConnectRequest"}
	if err := sendJSON(stream, msg); err != nil {
		log.Printf("❌ Ошибка при отправке bootstrap-сообщения: %v", err)
	}
}

func (a *Libp2pAgent) handleStream(s network.Stream) {
	log.Printf("📥 Входящее соединение от %s", s.Conn().RemotePeer().String())

	reader := bufio.NewReader(s)
	line, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("❌ Ошибка чтения из потока: %v", err)
		return
	}

	var msg Message
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &msg); err != nil {
		log.Printf("❌ Ошибка разбора JSON: %v", err)
		return
	}

	log.Printf("📨 Получено сообщение: %+v", msg)

	// Обработка по типу
	switch msg.Type {
	case "ConnectRequest":
		response := Message{Type: "Connected"}
		sendJSON(s, response)
	default:
		log.Printf("⚠️ Неизвестный тип сообщения: %s", msg.Type)
	}
}

func sendJSON(s network.Stream, msg Message) error {
	encoder := json.NewEncoder(s)
	return encoder.Encode(msg)
}

func (a *Libp2pAgent) Stop() {
	a.cancel()
	if err := a.Host.Close(); err != nil {
		log.Printf("⚠️ Ошибка при остановке агента: %v", err)
	}
}
