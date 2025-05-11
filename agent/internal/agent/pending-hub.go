package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"

	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/ruslanonly/agent/internal/agent/protocols/pendinghubproto"
	pendinghubprotomessages "github.com/ruslanonly/agent/internal/agent/protocols/pendinghubproto/messages"
	"github.com/ruslanonly/agent/internal/fsm"
)

func (a *Agent) startPendingHubStream() {
	log.Println("⚡️ Установлен обработчик сообщений pending-hub протокола")

	a.node.SetStreamHandler(pendinghubproto.ProtocolID, a.pendingHubStreamHandler)
}

func (a *Agent) closePendingHubStream() {
	log.Println("⚡️ Удален обработчик сообщений pending-hub протокола")
	a.node.RemoveStreamHandler(pendinghubproto.ProtocolID)
}

func (a *Agent) pendingHubStreamHandler(stream libp2pNetwork.Stream) {
	buf := bufio.NewReader(stream)
	raw, err := buf.ReadString('\n')

	if err != nil {
		log.Println(buf)
		log.Fatalf("⚡️ Ошибка при обработке потока сообщений для pending-hub протокола: %v", err)
	}

	log.Printf("⚡️ Получено сообщение по pending-hub протоколу: %s", raw)

	var msg pendinghubprotomessages.Message
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Printf("⚡️ Ошибка при парсинге сообщения: %v", err)
		return
	}

	if msg.Type == pendinghubprotomessages.TryConnectToMeMessageType {
		a.fsm.Event(
			fsm.RequestConnectionFromAbonentToHubAgentFSMEvent,
			stream.Conn().RemoteMultiaddr().String(),
			stream.Conn().RemotePeer().String(),
		)
	}
}

func (a *Agent) getPendingHubPeers() []peer.AddrInfo {
	setRaw, found := a.fsm.FSM.Metadata("pendingHubPeers")
	if !found {
		return nil
	}

	pendingHubPeers := setRaw.(map[peer.ID]peer.AddrInfo)
	peers := make([]peer.AddrInfo, 0, len(pendingHubPeers))
	for _, addr := range pendingHubPeers {
		peers = append(peers, addr)
	}
	return peers
}

func (a *Agent) addPendingHubPeer(addrInfo peer.AddrInfo) {
	setRaw, found := a.fsm.FSM.Metadata("pendingHubPeers")
	var pendingHubPeers map[peer.ID]peer.AddrInfo

	if !found {
		pendingHubPeers = make(map[peer.ID]peer.AddrInfo)
		a.fsm.FSM.SetMetadata("pendingHubPeers", pendingHubPeers)
	} else {
		pendingHubPeers = setRaw.(map[peer.ID]peer.AddrInfo)
	}

	pendingHubPeers[addrInfo.ID] = addrInfo
}

func (a *Agent) removePendingHubPeer(peerID peer.ID) {
	setRaw, found := a.fsm.FSM.Metadata("pendingHubPeers")
	if found {
		pendingHubPeers := setRaw.(map[peer.ID]peer.AddrInfo)
		delete(pendingHubPeers, peerID)
	}
}

// [HUB]
func (a *Agent) informPendingHubPeersToConnect() {
	pendingPeers := a.getPendingHubPeers()

	if len(pendingPeers) == 0 {
		return
	}

	message := pendinghubprotomessages.Message{
		Type: pendinghubprotomessages.TryConnectToMeMessageType,
	}

	for _, pendingPeer := range pendingPeers {
		connectedness := a.node.Host.Network().Connectedness(pendingPeer.ID)
		if connectedness != libp2pNetwork.Connected {
			err := a.node.Connect(pendingPeer)
			if err != nil {
				continue
			}
		}

		s, err := a.node.Host.NewStream(context.Background(), pendingPeer.ID, pendinghubproto.ProtocolID)
		if err != nil {
			log.Println(err)
			continue
		}

		fmt.Printf("❤️‍🔥 Информирование о новом хабе. Попробуй подключиться ко мне (%s)\n", pendingPeer.ID)

		if err := json.NewEncoder(s).Encode(message); err != nil {
			log.Println("Ошибка при отправке запроса:", err)
			continue
		}

		a.removePendingHubPeer(pendingPeer.ID)

		s.Close()
	}
}
