package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"slices"

	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/ruslanonly/agent/internal/agent/model/status"
	"github.com/ruslanonly/agent/internal/agent/protocols/hubproto"
	hubprotomessages "github.com/ruslanonly/agent/internal/agent/protocols/hubproto/messages"
	"github.com/ruslanonly/agent/internal/fsm"
	"github.com/ruslanonly/agent/internal/network"
)

func (a *Agent) hubMessage(messageType hubprotomessages.MessageType, body hubprotomessages.MessageBody) {
	a.peersMutex.RLock()
	defer a.peersMutex.RUnlock()

	message := hubprotomessages.Message{
		FromID:  a.node.Host.ID(),
		Type:    messageType,
		Body:    body,
		Visited: []peer.ID{a.node.Host.ID()},
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
	}
}

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

	for _, visited := range message.Visited {
		if visited == a.node.Host.ID() {
			log.Println("🔁 Сообщение уже обработано, пропускаем")
			return
		}
	}

	hubs, _ := a.getSplittedPeers()
	hubIDs := make([]peer.ID, 0)
	for _, hub := range hubs {
		if slices.Contains(message.Visited, hub.ID) || hub.ID == message.FromID {
			continue
		}

		hubIDs = append(hubIDs, hub.ID)
	}

	message.Visited = append(message.Visited, a.node.Host.ID())

	if marshalledMessage, err := json.Marshal(message); err != nil {
		log.Println("Ошибка при маршалинге broadcast-сообщения о красном трафике среди хабов:", err)
	} else {
		a.node.BroadcastToPeers(hubproto.ProtocolID, hubIDs, marshalledMessage)
	}

	if message.Type == hubprotomessages.RedTrafficMessageType {
		a.redTrafficHandler(message)
	} else if message.Type == hubprotomessages.InfoAboutHubMessageType {
		var infoAboutMe hubprotomessages.InfoAboutHubMessageBody
		if err := json.Unmarshal([]byte(message.Body), &infoAboutMe); err != nil {
			log.Println("Ошибка при парсинге ответа:", err)
			return
		}

		a.infoAboutMeHandler(infoAboutMe)
	}
}

func (a *Agent) infoAboutMeHandler(info hubprotomessages.InfoAboutHubMessageBody) {
	a.peersMutex.RLock()
	defer a.peersMutex.RUnlock()

	peerID, err := peer.Decode(info.ID)
	if err != nil {
		log.Println("❌ Не удалось декодировать info.ID в peer.ID:", info.ID, err)
		return
	}

	infoP2PStatus := info.Status.ToPeerP2PStatus()

	p, isMyPeer := a.peers[peerID]
	if isMyPeer {
		a.informPendingHubPeersToConnect()

		if p.Status.IsAbonent() {
			p.Status = infoP2PStatus

			a.peers[peerID] = p
			log.Printf("☝️ Пир %s стал хабом", peerID)

			peerIDs := a.fsm.GetElectionPeers()
			fmt.Println("PEER IDS", peerIDs)

			if slices.Contains(peerIDs, peerID) {
				a.fsm.Event(fsm.OrganizingSegmentHubElectionIsCompletedAgentFSMEvent)
			}
		}
	} else {
		s := infoP2PStatus

		a.fsm.AddKnownHub(fsm.KnownHub{
			ID:     peerID,
			Addrs:  info.Addrs,
			Status: s,
		})

		if s == status.HubFreeP2PStatus || s == status.HubFullHavingAbonentsP2PStatus {
			a.informPendingHubPeersToConnect()
		}
	}
}

// [HUB]
func (a *Agent) redTrafficHandler(message hubprotomessages.Message) {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	var offenderIP net.IP = net.IP(message.Body)

	log.Printf("🚨 Получено сообщение о красном трафике от IP: %s", offenderIP)

	a.threatsIPC.BlockHostMessage(offenderIP)

	a.broadcastBlockTrafficToAbonents(offenderIP)
}

func (a *Agent) RedTrafficHubMessage(offenderIP net.IP) {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	a.hubMessage(
		hubprotomessages.RedTrafficMessageType,
		hubprotomessages.MessageBody(offenderIP),
	)

}

// [HUB] Отправление информации о себе хабам
func (a *Agent) InfoAboutMeHubMessage() {
	fmt.Printf("🟪 Подготовка к информированию обо мне \n")
	a.peersMutex.RLock()
	defer a.peersMutex.RUnlock()
	fmt.Printf("🟪 Информирование обо мне\n")

	status := a.getHubSlotsStatus()

	hubs, _ := a.getSplittedPeers()
	hubsPeerIDs := make([]peer.ID, 0)

	for peerID := range hubs {
		hubsPeerIDs = append(hubsPeerIDs, peerID)
	}

	if len(hubsPeerIDs) == 0 {
		return
	}

	infoAboutMe := hubprotomessages.InfoAboutHubMessageBody{
		ID:     a.node.Host.ID().String(),
		Addrs:  network.MultiaddrsToMultiaddrStrs(a.node.Host.Addrs()),
		Status: status,
	}

	if marshalledBody, err := json.Marshal(infoAboutMe); err != nil {
		log.Println("Ошибка при маршалинге тела информации о себе (хабе):", err)
		return
	} else {
		a.hubMessage(hubprotomessages.InfoAboutHubMessageType, marshalledBody)
	}
}
