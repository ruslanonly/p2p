package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"

	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/ruslanonly/agent/internal/agent/protocols/threatsproto"
	threatsprotomessages "github.com/ruslanonly/agent/internal/agent/protocols/threatsproto/messages"
)

func (a *Agent) startThreatsStream() {
	log.Println("🔳 Установлен обработчик сообщений threats протокола")

	a.node.SetStreamHandler(threatsproto.ProtocolID, a.threatsStreamHandler)
}

func (a *Agent) closeThreatsStream() {
	log.Println("🔳 Удален обработчик сообщений threats протокола")

	a.node.RemoveStreamHandler(threatsproto.ProtocolID)
}

func (a *Agent) threatsStreamHandler(stream libp2pNetwork.Stream) {
	buf := bufio.NewReader(stream)
	raw, err := buf.ReadString('\n')

	if err != nil {
		fmt.Printf("🔳 Ошибка при обработке потока сообщений для threats протокола: %v\n", err)
		stream.Close()
		return
	}

	var message threatsprotomessages.Message
	if err := json.Unmarshal([]byte(raw), &message); err != nil {
		log.Printf("🔳 Ошибка при парсинге сообщения: %v\n", err)
		return
	}

	if message.Type == threatsprotomessages.BlockTrafficMessageType {
		log.Printf("🔳 Получено сообщение: необходимо заблокировать %s", message.IP)
		a.threatsIPC.BlockHostMessage(message.IP)
	}

	stream.Close()
}

// [ABONENT]
func (a *Agent) informMyHubAboutRedTraffic(ip net.IP) {
	myHub, found := a.getMyHub()
	if !found {
		return
	}

	log.Printf("🔳 Отправка информации о красном трафике своему хабу %s", myHub.ID)
	s, err := a.node.Host.NewStream(context.Background(), myHub.ID, threatsproto.ProtocolID)
	if err != nil {
		return
	}

	message := threatsprotomessages.Message{
		Type: threatsprotomessages.RedTrafficMessageType,
		IP:   ip,
	}

	if err := json.NewEncoder(s).Encode(message); err != nil {
		log.Printf("Ошибка при отправке уведомления о красном трафике: %v\n", err)
		return
	}

	s.Close()
}

// [ABONENT]
func (a *Agent) informMyHubAboutYellowTraffic(ip net.IP) {
	myHub, found := a.getMyHub()
	if !found {
		return
	}

	log.Printf("🔳 Отправка информации о желтом трафике своему хабу %s", myHub.ID)
	s, err := a.node.Host.NewStream(context.Background(), myHub.ID, threatsproto.ProtocolID)
	if err != nil {
		return
	}

	message := threatsprotomessages.Message{
		Type: threatsprotomessages.YellowTrafficMessageType,
		IP:   ip,
	}

	if err := json.NewEncoder(s).Encode(message); err != nil {
		log.Printf("Ошибка при отправке уведомления об желтом трафике: %v\n", err)
		return
	}

	s.Close()
}

// [HUB]
func (a *Agent) broadcastBlockTrafficToAbonents(offenderIP net.IP) {
	message := threatsprotomessages.Message{
		IP:   offenderIP,
		Type: threatsprotomessages.BlockTrafficMessageType,
	}

	log.Printf("🟪 Отправка информации о красном трафике своим абонентам %s", offenderIP)

	_, abonents := a.getSplittedPeers()
	peerIDs := make([]peer.ID, 0)
	for _, abonent := range abonents {
		peerIDs = append(peerIDs, abonent.ID)
	}

	if marshalledMessage, err := json.Marshal(message); err != nil {
		log.Println("Ошибка при маршалинге информации о блокировке красного трафика:", err)
	} else {
		a.node.BroadcastToPeers(threatsproto.ProtocolID, peerIDs, marshalledMessage)
	}
}
