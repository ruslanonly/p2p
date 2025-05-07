package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net"

	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
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
		log.Println(buf)
		log.Fatalf("🔳 Ошибка при обработке потока сообщений для threats протокола: %v\n", err)
	}

	var message threatsprotomessages.Message
	if err := json.Unmarshal([]byte(raw), &message); err != nil {
		log.Printf("🔳 Ошибка при парсинге сообщения: %v\n", err)
		return
	}

	if message.Type == threatsprotomessages.BlockTrafficMessageType {
		a.threatsIPC.BlockHostMessage(message.IP)
	}

	stream.Close()
}

func (a *Agent) informMyHubAboutRedTraffic(ip net.IP) {
	myHub, found := a.getMyHub()
	if !found {
		return
	}

	s, err := a.node.Host.NewStream(context.Background(), myHub.ID, threatsproto.ProtocolID)
	if err != nil {
		return
	}

	message := threatsprotomessages.Message{
		Type: threatsprotomessages.RedTrafficMessageType,
		IP:   ip,
	}

	if err := json.NewEncoder(s).Encode(message); err != nil {
		log.Printf("Ошибка при отправке уведомления об отключении: %v\n", err)
		return
	}

	s.Close()
}
