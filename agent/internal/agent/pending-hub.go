package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"

	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
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
		fmt.Printf("⚡️ Ошибка при обработке потока сообщений для pending-hub протокола: %v\n", err)
		stream.Close()
		return
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

// [HUB]
func (a *Agent) informPendingHubPeersToConnect() {
	pendingPeers := a.fsm.GetPendingHubPeers()

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

		a.fsm.RemovePendingHubPeer(pendingPeer.ID)

		s.Close()
	}
}
