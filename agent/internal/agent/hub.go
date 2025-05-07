package agent

import (
	"bufio"
	"encoding/json"
	"log"
	"net"

	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/ruslanonly/agent/internal/agent/protocols/hubproto"
	hubprotomessages "github.com/ruslanonly/agent/internal/agent/protocols/hubproto/messages"
	"github.com/ruslanonly/agent/internal/agent/protocols/threatsproto"
	threatsprotomessages "github.com/ruslanonly/agent/internal/agent/protocols/threatsproto/messages"
)

func (a *Agent) startHubStream() {
	log.Println("🟪 Установлен обработчик сообщений hub протокола")

	a.node.SetStreamHandler(hubproto.ProtocolID, a.hubStreamHandler)
}

func (a *Agent) hubStreamHandler(stream libp2pNetwork.Stream) {
	buf := bufio.NewReader(stream)
	raw, err := buf.ReadString('\n')

	if err != nil {
		log.Println(buf)
		log.Fatalf("🟪 Ошибка при обработке потока сообщений для hub протокола: %v\n", err)
	}

	var message hubprotomessages.Message
	if err := json.Unmarshal([]byte(raw), &message); err != nil {
		log.Printf("🟪 Ошибка при парсинге сообщения: %v\n", err)
		return
	}

	if message.Type == hubprotomessages.RedTrafficMessageType {
		a.redTrafficHandler(message)
	}
}

func (a *Agent) BroadcastRedTrafficHubMessage(offenderIP net.IP) {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	message := hubprotomessages.Message{
		FromID:  a.node.Host.ID(),
		Type:    hubprotomessages.RedTrafficMessageType,
		Payload: offenderIP,
		Visited: make([]peer.ID, 0),
	}

	hubs, _ := a.getSplittedPeers()
	hubIDs := make([]peer.ID, 0)
	for _, hub := range hubs {
		hubIDs = append(hubIDs, hub.ID)
	}

	if marshalledMessage, err := json.Marshal(message); err != nil {
		log.Println("Ошибка при маршалинге broadcast-сообщения о красном трафике среди хабов:", err)
	} else {
		a.node.BroadcastToPeers(hubproto.ProtocolID, hubIDs, marshalledMessage)
		a.broadcastRedTrafficToAbonents(offenderIP)
	}
}

func (a *Agent) redTrafficHandler(message hubprotomessages.Message) {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	for _, visited := range message.Visited {
		if visited == a.node.Host.ID() {
			log.Println("🔁 Сообщение уже обработано, пропускаем")
			return
		}
	}

	var offenderIP net.IP = message.Payload

	log.Printf("🚨 Получено сообщение о красном трафике от IP: %s", offenderIP.String())

	a.threatsIPC.BlockHostMessage(offenderIP)

	message.Visited = append(message.Visited, a.node.Host.ID())

	hubs, _ := a.getSplittedPeers()
	hubIDs := make([]peer.ID, 0)
	for _, hub := range hubs {
		hubIDs = append(hubIDs, hub.ID)
	}

	if marshalledMessage, err := json.Marshal(message); err != nil {
		log.Println("Ошибка при маршалинге broadcast-сообщения о красном трафике среди хабов:", err)
	} else {
		a.node.BroadcastToPeers(hubproto.ProtocolID, hubIDs, marshalledMessage)
		a.broadcastRedTrafficToAbonents(offenderIP)
	}
}

func (a *Agent) broadcastRedTrafficToAbonents(offenderIP net.IP) {
	message := threatsprotomessages.Message{
		IP:   offenderIP,
		Type: threatsprotomessages.BlockTrafficMessageType,
	}

	hubs, _ := a.getSplittedPeers()
	peerIDs := make([]peer.ID, 0)
	for _, hub := range hubs {
		peerIDs = append(peerIDs, hub.ID)
	}

	if marshalledMessage, err := json.Marshal(message); err != nil {
		log.Println("Ошибка при маршалинге информации о блокировке красного трафика:", err)
	} else {
		a.node.BroadcastToPeers(threatsproto.ProtocolID, peerIDs, marshalledMessage)
	}
}
