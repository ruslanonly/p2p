package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"log"

	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
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
		log.Println(buf)
		log.Fatalf("❤️ Ошибка при обработке потока сообщений для heartbeat протокола: %v\n", err)
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
			a.disconnectPeer(peerInfo.ID, false)
			return
		}

		s, err := a.node.Host.NewStream(context.Background(), peerInfo.ID, heartbeatproto.ProtocolID)
		if err != nil {
			a.disconnectPeer(peerInfo.ID, false)
			return
		}

		if _, err := s.Write(append(marshalledMessage, '\n')); err != nil {
			a.disconnectPeer(peerInfo.ID, false)
		}

		s.Close()
	}
}
