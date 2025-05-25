package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"

	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/ruslanonly/agent/internal/agent/protocols/heartbeatproto"
	heartbeatprotomessages "github.com/ruslanonly/agent/internal/agent/protocols/heartbeatproto/messages"
)

func (a *Agent) startHeartbeatStream() {
	log.Println("❤️ Установлен обработчик сообщений heartbeat протокола")

	a.node.SetStreamHandler(heartbeatproto.ProtocolID, a.heartbeatStreamHandler)
}

func (a *Agent) heartbeatStreamHandler(stream libp2pNetwork.Stream) {
	buf := bufio.NewReader(stream)
	raw, err := buf.ReadString('\n')

	if err != nil {
		fmt.Printf("❤️ Ошибка при обработке потока сообщений для heartbeat протокола: %v\n", err)
		stream.Close()
		return
	}

	var msg heartbeatprotomessages.Message
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Printf("❤️ Ошибка при парсинге сообщения: %v\n", err)
		return
	}

	if msg.Type == heartbeatprotomessages.CheckHeartbeatMessageType {
		message := heartbeatprotomessages.Message{
			Type: heartbeatprotomessages.BoomBoomMessageType,
		}

		if err := json.NewEncoder(stream).Encode(message); err != nil {
			log.Printf("❤️ Ошибка при отправке ответа на проверку heartbeat: %v\n", err)
		}
	}
}

func (a *Agent) checkAllPeersHeartbeat() {
	message := heartbeatprotomessages.Message{
		Type: heartbeatprotomessages.CheckHeartbeatMessageType,
	}

	marshalledMessage, err := json.Marshal(message)
	if err != nil {
		log.Printf("❤️ Ошибка при маршалинге сообщения-запроса на проверку heartbeat: %v\n", err)
		return
	}

	for _, peerInfo := range a.peers {
		// log.Printf("❤️❤️❤️: %s\n", peerInfo.ID)
		connections := a.node.Host.Network().ConnsToPeer(peerInfo.ID)
		if len(connections) == 0 {
			a.handleUnreachablePeer(peerInfo.ID)
			return
		}

		s, err := a.node.Host.NewStream(context.Background(), peerInfo.ID, heartbeatproto.ProtocolID)
		if err != nil {
			a.handleUnreachablePeer(peerInfo.ID)
			return
		}

		if _, err := s.Write(append(marshalledMessage, '\n')); err != nil {
			a.handleUnreachablePeer(peerInfo.ID)
		}

		s.Close()
	}
}

func (a *Agent) checkPeerHeartbeat(peerID peer.ID) bool {
	message := heartbeatprotomessages.Message{
		Type: heartbeatprotomessages.CheckHeartbeatMessageType,
	}

	marshalledMessage, err := json.Marshal(message)
	if err != nil {
		log.Printf("❤️ Ошибка при маршалинге сообщения-запроса на проверку heartbeat: %v\n", err)
		return false
	}

	connections := a.node.Host.Network().ConnsToPeer(peerID)
	if len(connections) == 0 {
		a.handleUnreachablePeer(peerID)
		return false
	}

	s, err := a.node.Host.NewStream(context.Background(), peerID, heartbeatproto.ProtocolID)
	if err != nil {
		a.handleUnreachablePeer(peerID)
		return false
	}

	if _, err := s.Write(append(marshalledMessage, '\n')); err != nil {
		a.handleUnreachablePeer(peerID)
	}

	s.Close()

	return false
}

func (a *Agent) handleUnreachablePeer(peerID peer.ID) {
	peer, peerFound := a.peers[peerID]
	if peerFound {
		if peer.Status.IsHub() {
			// Необходимо организовать выборы и после подключиться к новому пиру

		} else {

		}
	}

	a.disconnectPeer(peerID, false)
}
