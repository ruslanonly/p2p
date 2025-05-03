package network

import (
	"context"
	"fmt"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
)

type LibP2PNode struct {
	Host host.Host
	ctx  context.Context
}

func NewLibP2PNode(ctx context.Context, port int) (*LibP2PNode, error) {
	connManager, err := connmgr.NewConnManager(2, 3, connmgr.WithGracePeriod(time.Second))

	if err != nil {
		return nil, err
	}

	ip4tcp := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)
	ip4udp := fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", port)

	h, err := libp2p.New(
		libp2p.ListenAddrStrings(ip4tcp, ip4udp),
		libp2p.ConnectionManager(connManager),
	)

	if err != nil {
		return nil, err
	}

	n := &LibP2PNode{
		Host: h,
		ctx:  ctx,
	}

	return n, nil
}

func (n *LibP2PNode) PeerAddrs(ID peer.ID) []string {
	multiaddrs := n.Host.Peerstore().PeerInfo(ID).Addrs

	addrs := make([]string, 0)

	for _, ma := range multiaddrs {
		addrs = append(addrs, ma.String())
	}

	return addrs
}

func (n *LibP2PNode) SetStreamHandler(protocolID protocol.ID, handler network.StreamHandler) {
	n.Host.SetStreamHandler(protocolID, handler)
}

func (n *LibP2PNode) RemoveStreamHandler(protocolID protocol.ID) {
	n.Host.RemoveStreamHandler(protocolID)
}

func (n *LibP2PNode) Connect(peerInfo peer.AddrInfo) error {
	return n.Host.Connect(n.ctx, peerInfo)
}

// One way
func (n *LibP2PNode) Stream(protocolID protocol.ID, peerID peer.ID, msg []byte) error {
	stream, err := n.Host.NewStream(n.ctx, peerID, protocolID)
	if err != nil {
		fmt.Printf("Ошибка открытия протокола %s: %v", protocolID, err)
		return err
	}

	_, err = stream.Write(msg)
	if err != nil {
		fmt.Println("Ошибка отправление сообщения:", err)
		return err
	}
	_ = stream.Close()

	return nil
}

func (n *LibP2PNode) BroadcastToPeers(protocolID protocol.ID, peers []peer.ID, msg []byte) {
	for _, peerID := range peers {
		if peerID == n.Host.ID() {
			continue
		}

		stream, err := n.Host.NewStream(n.ctx, peerID, protocolID)
		if err != nil {
			fmt.Printf("Ошибка открытия протокола %s: %v", protocolID, err)
			continue
		}

		_, err = stream.Write(append(msg, '\n'))
		if err != nil {
			fmt.Println("Ошибка отправление сообщения:", err)
		}
		_ = stream.Close()
	}
}

func (n *LibP2PNode) PrintHostInfo() {
	out := fmt.Sprintf("Peer ID: %s\n", n.Host.ID().String())

	for _, addr := range n.Host.Addrs() {
		out += fmt.Sprintf("Listening on: %s/p2p/%s\n", addr, n.Host.ID().String())
	}

	fmt.Print(out)
}

func (n *LibP2PNode) Close() error {
	return n.Host.Close()
}
