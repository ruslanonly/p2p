package network

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type Notifiee struct {
	OnDisconnect func(peer.ID)
}

func (n *Notifiee) Listen(net network.Network, addr ma.Multiaddr)           {}
func (n *Notifiee) ListenClose(net network.Network, addr ma.Multiaddr)      {}
func (n *Notifiee) Connected(net network.Network, conn network.Conn)        {}
func (n *Notifiee) OpenedStream(net network.Network, stream network.Stream) {}
func (n *Notifiee) ClosedStream(net network.Network, stream network.Stream) {}

func (n *Notifiee) Disconnected(net network.Network, conn network.Conn) {
	peerID := conn.RemotePeer()
	if n.OnDisconnect != nil {
		n.OnDisconnect(peerID)
	}
}
