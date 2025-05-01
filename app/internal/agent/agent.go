package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftnet "github.com/libp2p/go-libp2p-raft"
	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	looplabFSM "github.com/looplab/fsm"
	"github.com/multiformats/go-multiaddr"
	"github.com/ruslanonly/p2p/internal/agent/messages"
	"github.com/ruslanonly/p2p/internal/consensus/hubelection"
	"github.com/ruslanonly/p2p/internal/fsm"
	"github.com/ruslanonly/p2p/internal/network"
)

type AgentPeerInfoPeer struct {
	ID    peer.ID
	Addrs []string
	IsHub bool
}

type AgentPeerInfo struct {
	ID    peer.ID
	IsHub bool
	Peers map[peer.ID]AgentPeerInfoPeer
}

type Agent struct {
	node *network.LibP2PNode
	ctx  context.Context
	fsm  *fsm.AgentFSM

	// Подключенные абоненты и хабы
	peers      map[peer.ID]AgentPeerInfo
	peersMutex sync.RWMutex

	// Максимальное количество подключенных абонентов и хабов
	peersLimit int
}

type StartOptions struct {
	BootstrapIP     string
	BootstrapPeerID string
}

func NewAgent(ctx context.Context, peersLimit, port int) (*Agent, error) {
	libp2pNode, err := network.NewLibP2PNode(ctx, port)

	if err != nil {
		log.Fatalf("Возникла ошибка при инициализации агента: %v", err)
	}

	agent := &Agent{
		node:       libp2pNode,
		ctx:        ctx,
		peers:      make(map[peer.ID]AgentPeerInfo),
		peersLimit: peersLimit,
	}

	return agent, nil
}

func (a *Agent) getSplittedPeers() (map[peer.ID]AgentPeerInfo, map[peer.ID]AgentPeerInfo) {
	hubs := make(map[peer.ID]AgentPeerInfo)
	abonents := make(map[peer.ID]AgentPeerInfo)

	for peerID, peerInfo := range a.peers {
		if peerInfo.IsHub {
			hubs[peerID] = peerInfo
		} else {
			abonents[peerID] = peerInfo
		}
	}

	return hubs, abonents
}

// [ABONENT]
func (a *Agent) getMyHub() (*AgentPeerInfo, bool) {
	for _, peerInfo := range a.peers {
		if peerInfo.IsHub {
			return &peerInfo, true
		}
	}

	return nil, false
}

// [ABONENT]
func (a *Agent) getSegmentPeers() map[peer.ID]AgentPeerInfoPeer {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	myHub, myHubIsFound := a.getMyHub()
	segmentPeers := make(map[peer.ID]AgentPeerInfoPeer)

	if myHubIsFound {
		for peerID, peerInfo := range a.peers[myHub.ID].Peers {
			if !peerInfo.IsHub {
				segmentPeers[peerID] = peerInfo
			}
		}
	}

	return segmentPeers
}

func (a *Agent) isPeersLimitExceeded() bool {
	out := len(a.peers) >= a.peersLimit

	return out
}

func (a *Agent) getHubSlotsStatus() messages.HubSlotsStatus {
	abonents, _ := a.getSplittedPeers()

	hasAbonents := len(abonents) == 0
	var status messages.HubSlotsStatus

	if !a.isPeersLimitExceeded() {
		status = messages.FreeHubSlotsStatus
	} else if hasAbonents {
		status = messages.FullHavingAbonentsHubSlotsStatus
	} else {
		status = messages.FullNotHavingAbonentsHubSlotsStatus
	}

	return status
}

func (a *Agent) disconnectPeer(peerID peer.ID) {
	for _, conn := range a.node.Host.Network().ConnsToPeer(peerID) {
		_ = conn.Close()
	}
	a.node.Host.Peerstore().RemovePeer(peerID)
	a.node.Host.ConnManager().Unprotect(peerID, "permanent")
	delete(a.peers, peerID)
}

func (a *Agent) disconnectAllPeers() {
	for peerID := range a.peers {
		a.disconnectPeer(peerID)
	}

	a.peers = make(map[peer.ID]AgentPeerInfo)
}

func (a *Agent) Start(options *StartOptions) {
	a.node.PrintHostInfo()

	a.fsm = fsm.NewAgentFSM(
		a.ctx,
		looplabFSM.Callbacks{
			"enter_state": func(e_ context.Context, e *looplabFSM.Event) {
				log.Printf("📦 FSM переход: %s -> %s по событию '%s' с аргументами %s", e.Src, e.Dst, e.Event, e.Args)
			},
			fsm.ConnectingToHubAgentFSMState: func(e_ context.Context, e *looplabFSM.Event) {
				bootstrapAddr, ok1 := e.Args[0].(string)
				if !ok1 {
					log.Printf("❌ Неверный первый аргумент для ConnectingToHubAgentFSMState: %v\n", e.Args[0])
					return
				}

				bootstrapPeerID, ok2 := e.Args[1].(string)
				if !ok2 {
					log.Printf("❌ Неверный второй аргумент для ConnectingToHubAgentFSMState: %v\n", e.Args[1])
					return
				}

				a.bootstrap(bootstrapAddr, bootstrapPeerID)
				e.FSM.SetMetadata(fsm.RoleAgentFSMMetadataKey, fsm.AbonentRole)
			},
			fsm.EnterStateFSMCallbackName(fsm.ListeningMessagesAsHubAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				e.FSM.SetMetadata(fsm.RoleAgentFSMMetadataKey, fsm.HubRole)
				a.startStream()

				infoAboutMeCtx, infoAboutMeCancelCtx := context.WithCancel(context.Background())

				e.FSM.SetMetadata("infoAboutMeCtx", infoAboutMeCtx)
				e.FSM.SetMetadata("infoAboutMeCancelCtx", infoAboutMeCancelCtx)

				go func(ctx context.Context) {
					ticker := time.NewTicker(20 * time.Second)
					defer ticker.Stop()

					a.broadcastToHubsInfoAboutMe()
					a.broadcastToSegmentInfoAboutSegment()

					for {
						select {
						case <-ctx.Done():
							log.Println("🛑 Цикл оповещения остановлен (отменён через cancel)")
							return
						case <-ticker.C:
							a.broadcastToHubsInfoAboutMe()
							a.broadcastToSegmentInfoAboutSegment()
						}
					}
				}(infoAboutMeCtx)
			},
			fsm.LeaveStateFSMCallbackName(fsm.ListeningMessagesAsHubAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {

			},
			fsm.EnterStateFSMCallbackName(fsm.ListeningMessagesAsAbonentAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				e.FSM.SetMetadata(fsm.RoleAgentFSMMetadataKey, fsm.AbonentRole)
				a.startStream()
			},
			fsm.OrganizingSegmentHubElectionAgentFSMState: func(e_ context.Context, e *looplabFSM.Event) {
				a.organizeSegmentHubElection()
			},
			fsm.EnterStateFSMCallbackName(fsm.ElectingNewHubAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				segmentPeers, ok := e.Args[0].([]AgentPeerInfoPeer)

				if !ok {
					log.Println("Первый аргумент peers должен иметь тип данных []AgentPeerInfoPeer")
					return
				}

				initialize, ok := e.Args[1].(bool)

				if !ok {
					log.Println("Второй аргумент initialize должен иметь тип данных bool")
					return
				}

				if initialize {
					a.initializeElectionForMySegment(segmentPeers)
				} else {
					a.prepareForElection(segmentPeers)
				}
			},
		},
	)

	if options != nil {
		bootstrapAddr := fmt.Sprintf("/ip4/%s/tcp/5000", options.BootstrapIP)

		// Узел начинает свою работу как обычный абонент
		a.fsm.Event(fsm.ReadInitialSettingsAgentFSMEvent, bootstrapAddr, options.BootstrapPeerID)
	} else {
		// Узел начинает свою работу как хаб
		a.fsm.Event(fsm.BecomeHubAgentFSMEvent)
	}

	notifiee := network.Notifiee{
		OnDisconnect: func(peerID peer.ID) {
			fmt.Println("❌ Отключение от :", peerID)
			a.disconnectPeer(peerID)
		},
	}

	a.node.Host.Network().Notify(&notifiee)

	<-a.ctx.Done()
	fmt.Println("Агент выключается...")
	_ = a.node.Close()
}

func (a *Agent) isHub() bool {
	raw, ok := a.fsm.FSM.Metadata(fsm.RoleAgentFSMMetadataKey)
	metadataRole, err := raw.(fsm.RoleAgentFSMMetadataValue)
	if !ok || err {
		log.Println("Возникла ошибка при обработке FSM Metadata Role")
	}

	return metadataRole == fsm.HubRole
}

func (a *Agent) isAbonent() bool {
	raw, ok := a.fsm.FSM.Metadata(fsm.RoleAgentFSMMetadataKey)
	metadataRole, err := raw.(fsm.RoleAgentFSMMetadataValue)
	if !ok || err {
		log.Println("Возникла ошибка при обработке FSM Metadata Role")
	}

	return metadataRole == fsm.AbonentRole
}

func (a *Agent) bootstrap(addr, peerID string) {
	period := 10 * time.Second

	a.disconnectAllPeers()

	addrWithPeerID := fmt.Sprintf("%s/p2p/%s", addr, peerID)
	maddr, err := multiaddr.NewMultiaddr(addrWithPeerID)
	if err != nil {
		log.Fatalf("Ошибка парсинга адреса bootstrap: %v", err)
	}

	log.Printf("Попытка подключиться к bootstrap-узлу: %s", maddr.String())

	for {
		hubAddrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Printf("Ошибка парсинга peer.AddrInfo: %v", err)
			time.Sleep(period)
			continue
		}

		if err := a.node.Connect(*hubAddrInfo); err != nil {
			log.Printf("Подключение к bootstrap не удалось: %v. Повтор через %s...", err, period)
			if strings.Contains(err.Error(), "peer id mismatch") {
				re := regexp.MustCompile(`remote key matches ([\w\d]+)`)
				matches := re.FindStringSubmatch(err.Error())
				if len(matches) > 1 {
					actualBootstrapPeerID := matches[1]
					log.Printf("⚠️ Обнаружен актуальный PeerID: %s", actualBootstrapPeerID)
					a.bootstrap(addr, actualBootstrapPeerID)
					break
				}
			}
			time.Sleep(period)
		} else {
			a.node.Host.ConnManager().Protect(hubAddrInfo.ID, "permanent")

			s, err := a.node.Host.NewStream(context.Background(), hubAddrInfo.ID, ProtocolID)
			if err != nil {
				log.Println(err)
				return
			}

			msg := messages.Message{
				Type: messages.ConnectRequestMessageType,
			}

			if err := json.NewEncoder(s).Encode(msg); err != nil {
				log.Println("Ошибка при отправке запрос на подключение:", err)
				return
			}

			reader := bufio.NewReader(s)
			responseRaw, err := reader.ReadString('\n')
			if err != nil {
				log.Println("Ошибка при чтении ответа:", err)
				return
			}

			var message messages.Message
			if err := json.Unmarshal([]byte(responseRaw), &message); err != nil {
				log.Println("Ошибка при парсинге сообщения:", err)
				return
			}

			if message.Type == messages.ConnectedMessageType {
				log.Printf("Я подключен к хабу")
				a.fsm.Event(fsm.ConnectedToHubAgentFSMEvent)

				var body messages.ConnectedMessageBody
				if err := json.Unmarshal(message.Body, &body); err != nil {
					log.Println("Ошибка при парсинге ответа:", err)
					return
				}

				a.peers[hubAddrInfo.ID] = AgentPeerInfo{
					ID:    hubAddrInfo.ID,
					IsHub: true,
					Peers: make(map[peer.ID]AgentPeerInfoPeer, 0),
				}

				a.handleInfoAboutSegment(hubAddrInfo.ID, body.Peers)
			} else if message.Type == messages.NotConnectedAndWaitMessageType {
				log.Print("Я не подключен, но ожидаю сообщения о новом хабе")
			} else if message.Type == messages.NotConnectedMessageType {
				log.Print("Я не подключен")
				a.fsm.Event(fsm.NotConnectedToHubAgentFSMEvent, addr, peerID)
			}

			break
		}
	}
}

func (a *Agent) startStream() {
	log.Println("Установлен обработчик сообщений для hub-потока")

	a.node.SetStreamHandler(ProtocolID, a.streamHandler)

	a.node.Host.Network().Notify(a.node.Host.ConnManager().Notifee())

}

func (a *Agent) closeStream() {
	a.node.RemoveStreamHandler(ProtocolID)
}

func (a *Agent) streamHandler(stream libp2pNetwork.Stream) {
	buf := bufio.NewReader(stream)
	raw, err := buf.ReadString('\n')

	if err != nil {
		log.Println(buf)
		log.Fatalf("Ошибка при обработке потока сообщений: %v", err)
	}

	log.Printf("Получено сообщение: %s", raw)

	var msg messages.Message
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Printf("Ошибка при парсинге сообщения: %v", err)
		return
	}

	if msg.Type == messages.ConnectRequestMessageType {
		a.handleConnectionRequestMessage(stream)
	} else if msg.Type == messages.BecomeOnlyOneHubMessageType {
		a.fsm.Event(fsm.BecomeHubAgentFSMEvent)
	} else if msg.Type == messages.InitializeElectionRequestMessageType {
		var body messages.InitializeElectionRequestMessageBody
		if err := json.Unmarshal([]byte(msg.Body), &body); err != nil {
			log.Printf("Ошибка при парсинге сообщения: %v", err)
			return
		}

		a.handleInfoAboutSegment(stream.Conn().RemotePeer(), body.Peers)

		segmentPeersMap := a.getSegmentPeers()
		segmentPeersArr := make([]AgentPeerInfoPeer, 0)
		for _, p := range segmentPeersMap {
			segmentPeersArr = append(segmentPeersArr, p)
		}

		log.Println("🚩 Я должен начать выборы")
		err := a.fsm.Event(fsm.ElectNewHubRequestFSMEvent, segmentPeersArr, true)
		if err != nil {
			log.Printf("Ошибка при FSM переходе: %v", err)
		}
	} else if msg.Type == messages.InfoAboutSegmentMessageType {
		var infoAboutSegment messages.InfoAboutSegmentMessageBody
		if err := json.Unmarshal([]byte(msg.Body), &infoAboutSegment); err != nil {
			log.Println("Ошибка при парсинге ответа:", err)
			return
		}

		a.handleInfoAboutSegment(stream.Conn().RemotePeer(), infoAboutSegment.Peers)
	} else if msg.Type == messages.ElectionRequestMessageType {
		log.Println("🚩 Меня позвали участвовать в выборах нового хаба")

		var infoAboutSegment messages.InfoAboutSegmentMessageBody
		if err := json.Unmarshal([]byte(msg.Body), &infoAboutSegment); err != nil {
			log.Println("Ошибка при парсинге ответа:", err)
			return
		}

		// TODO: COPY PASTE FROM msg.Type == messages.InfoAboutSegmentMessageType
		segmentPeersMap := a.getSegmentPeers()
		segmentPeersArr := make([]AgentPeerInfoPeer, 0)
		for _, p := range segmentPeersMap {
			segmentPeersArr = append(segmentPeersArr, p)
		}

		err := a.fsm.Event(fsm.ElectNewHubRequestFSMEvent, segmentPeersArr, false)
		if err != nil {
			log.Printf("Ошибка при FSM переходе: %v", err)
		}
	} else if msg.Type == messages.InfoAboutMeForHubsMessageType {
		var infoAboutHub messages.InfoAboutMeForHubsMessageBody
		if err := json.Unmarshal([]byte(msg.Body), &infoAboutHub); err != nil {
			log.Println("Ошибка при парсинге ответа:", err)
			return
		}

		a.handleInfoAboutHub(infoAboutHub)
	}
}

// [HUB] Обработка запроса на подключение
func (a *Agent) handleConnectionRequestMessage(stream libp2pNetwork.Stream) {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()
	slotsStatus := a.getHubSlotsStatus()

	if slotsStatus == messages.FreeHubSlotsStatus {
		a.handleConnectedOnConnectionRequest(stream)
	} else {
		var msg messages.Message

		if slotsStatus == messages.FullHavingAbonentsHubSlotsStatus {
			msg = messages.Message{
				Type: messages.NotConnectedAndWaitMessageType,
			}

			if a.fsm.FSM.Can(fsm.OrganizeSegmentHubElectionAgentFSMEvent) {
				a.fsm.Event(fsm.OrganizeSegmentHubElectionAgentFSMEvent)
			}
		} else {
			msg = messages.Message{
				Type: messages.NotConnectedMessageType,
			}

			// TODO: Поиск хабов среди известных
		}

		if err := json.NewEncoder(stream).Encode(msg); err != nil {
			log.Printf("Ошибка при отправке сообщения об неуспешном подключении узлу %s: %v\n", stream.Conn().RemotePeer(), err)
			return
		}

		stream.Close()
	}
}

// [HUB]
func (a *Agent) handleConnectedOnConnectionRequest(stream libp2pNetwork.Stream) {
	remotePeerID := stream.Conn().RemotePeer()

	_, abonents := a.getSplittedPeers()
	abonentsPeerInfos := make([]messages.InfoAboutSegmentPeerInfo, 0)

	for peerID, peerInfo := range abonents {
		addrs := a.node.PeerAddrs(peerID)

		abonentsPeerInfos = append(abonentsPeerInfos, messages.InfoAboutSegmentPeerInfo{
			ID:    peerID,
			IsHub: peerInfo.IsHub,
			Addrs: addrs,
		})
	}

	body := messages.ConnectedMessageBody{
		Peers: abonentsPeerInfos,
	}

	if marshaledBody, err := json.Marshal(body); err != nil {
		log.Println("Ошибка при маршалинге тела информации о себе:", err)
		return
	} else {
		infoAboutSegmentMessage := messages.Message{
			Type: messages.ConnectedMessageType,
			Body: marshaledBody,
		}

		if err := json.NewEncoder(stream).Encode(infoAboutSegmentMessage); err != nil {
			log.Printf("Ошибка при отправке сообщения об успешном подключении узла %s: %v\n", remotePeerID, err)
			return
		}
	}

	log.Printf("Подключен новый узел %s\n", remotePeerID)

	a.peers[remotePeerID] = AgentPeerInfo{
		ID:    remotePeerID,
		IsHub: false,
		Peers: nil,
	}
}

// [HUB] Отправление информации о себе хабам
func (a *Agent) broadcastToHubsInfoAboutMe() {
	a.peersMutex.RLock()
	defer a.peersMutex.RUnlock()

	status := a.getHubSlotsStatus()

	hubs, _ := a.getSplittedPeers()
	hubsPeerIDs := make([]peer.ID, 0)

	for peerID := range hubs {
		hubsPeerIDs = append(hubsPeerIDs, peerID)
	}

	if len(hubsPeerIDs) == 0 {
		return
	}

	infoAboutMe := messages.InfoAboutMeForHubsMessageBody{
		ID:     a.node.Host.ID().String(),
		Addrs:  network.MultiaddrsToMultiaddrStrs(a.node.Host.Addrs()),
		Status: status,
	}

	if marshaledBody, err := json.Marshal(infoAboutMe); err != nil {
		log.Println("Ошибка при маршалинге тела информации о себе:", err)
		return
	} else {

		infoAboutMeMessage := messages.Message{
			Type: messages.InfoAboutMeForHubsMessageType,
			Body: marshaledBody,
		}

		if marshaledMessage, err := json.Marshal(infoAboutMeMessage); err != nil {
			log.Println("Ошибка при маршалинге информации о себе:", err)
			return
		} else {
			log.Printf("Отправка broadcast-сообщение о себе")
			a.node.BroadcastToPeers(ProtocolID, hubsPeerIDs, marshaledMessage)
		}
	}
}

// [ABONENT]
func (a *Agent) handleInfoAboutHub(info messages.InfoAboutMeForHubsMessageBody) {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	peerID, err := peer.Decode(info.ID)
	if err != nil {
		log.Println("❌ Не удалось декодировать info.ID в peer.ID:", info.ID, err)
		return
	}

	p, found := a.peers[peerID]
	if !found {
		return
	}

	if !p.IsHub {
		p.IsHub = true
		a.peers[peerID] = p
		log.Printf("☝️ Пир %s стал хабом", info.ID)
	}
}

// [HUB]
func (a *Agent) broadcastToSegmentInfoAboutSegment() {
	a.peersMutex.RLock()
	defer a.peersMutex.RUnlock()

	_, abonents := a.getSplittedPeers()
	abonentsPeerIDs := make([]peer.ID, 0)
	abonentsPeerInfos := make([]messages.InfoAboutSegmentPeerInfo, 0)

	for peerID, peerInfo := range abonents {
		addrs := a.node.PeerAddrs(peerID)
		abonentsPeerIDs = append(abonentsPeerIDs, peerID)

		abonentsPeerInfos = append(abonentsPeerInfos, messages.InfoAboutSegmentPeerInfo{
			ID:    peerID,
			IsHub: peerInfo.IsHub,
			Addrs: addrs,
		})
	}

	if len(abonentsPeerIDs) == 0 {
		return
	}

	infoAboutSegment := messages.InfoAboutSegmentMessageBody{
		Peers: abonentsPeerInfos,
	}

	if marshaledBody, err := json.Marshal(infoAboutSegment); err != nil {
		log.Println("Ошибка при маршалинге тела информации о себе:", err)
		return
	} else {

		infoAboutSegmentMessage := messages.Message{
			Type: messages.InfoAboutSegmentMessageType,
			Body: marshaledBody,
		}

		if marshaledMessage, err := json.Marshal(infoAboutSegmentMessage); err != nil {
			log.Println("Ошибка при маршалинге информации о себе:", err)
			return
		} else {
			log.Printf("Отправка broadcast-сообщение о сегменте %v", abonentsPeerIDs)
			a.node.BroadcastToPeers(ProtocolID, abonentsPeerIDs, marshaledMessage)
		}
	}
}

// [ABONENT]
func (a *Agent) handleInfoAboutSegment(hubID peer.ID, peers []messages.InfoAboutSegmentPeerInfo) {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	if _, ok := a.peers[hubID]; !ok {
		log.Println("Это не мой хаб: ", hubID)
		return
	}

	for _, p := range peers {
		mas, err := network.MultiaddrsStrsToMultiaddrs(p.Addrs)

		if err != nil {
			log.Printf("Возникла ошибка при обработке адресов пира %s: %v", p.ID, err)
			continue
		}

		connectedness := a.node.Host.Network().Connectedness(p.ID)

		if connectedness == libp2pNetwork.Connected || p.ID == a.node.Host.ID() {
			a.peers[hubID].Peers[p.ID] = AgentPeerInfoPeer{
				ID:    p.ID,
				Addrs: p.Addrs,
				IsHub: p.IsHub,
			}
		} else {
			info := peer.AddrInfo{
				ID:    p.ID,
				Addrs: mas,
			}

			if err := a.node.Connect(info); err != nil {
				log.Printf("Возникла ошибка при подключении пира %s: %v", p.ID, err)
			} else {
				a.node.Host.ConnManager().Protect(info.ID, "permanent")
				a.peers[hubID].Peers[p.ID] = AgentPeerInfoPeer{
					ID:    p.ID,
					Addrs: p.Addrs,
					IsHub: p.IsHub,
				}
			}
		}
	}

	log.Printf("🟦 Мои пиры %d --- %v", len(a.peers), a.peers)
}

// [HUB]
func (a *Agent) organizeSegmentHubElection() {
	_, abonents := a.getSplittedPeers()

	if len(abonents) < 1 {
		return
	} else {
		var abonent AgentPeerInfo
		for _, a := range abonents {
			abonent = a
			break
		}

		var message messages.Message
		if len(abonents) == 1 {
			// Абонент становится хабом сразу, если он единственный абонент в сегменте
			log.Printf("Отправка сообщения о необходимости стать единственным хабом")
			message = messages.Message{
				Type: messages.BecomeOnlyOneHubMessageType,
			}
		} else {
			// Первый абонент из списка абонентов должен являться инициатором выборов среди другого сегмента, о котором он знает
			log.Printf("Отправка сообщения о необходимости инициализировать выборы среди абонентов сегмента")

			_, abonents := a.getSplittedPeers()
			abonentsPeerInfos := make([]messages.InfoAboutSegmentPeerInfo, 0)

			for peerID, peerInfo := range abonents {
				addrs := a.node.PeerAddrs(peerID)

				abonentsPeerInfos = append(abonentsPeerInfos, messages.InfoAboutSegmentPeerInfo{
					ID:    peerID,
					IsHub: peerInfo.IsHub,
					Addrs: addrs,
				})
			}

			body := messages.InitializeElectionRequestMessageBody{
				Peers: abonentsPeerInfos,
			}

			if marshaledBody, err := json.Marshal(body); err != nil {
				log.Println("Ошибка при маршалинге тела InitializeElectionRequestMessageBody:", err)
				return
			} else {
				message = messages.Message{
					Type: messages.InitializeElectionRequestMessageType,
					Body: marshaledBody,
				}
			}

		}

		s, err := a.node.Host.NewStream(context.Background(), abonent.ID, ProtocolID)
		if err != nil {
			log.Println(err)
			return
		}

		if err := json.NewEncoder(s).Encode(message); err != nil {
			log.Println("Ошибка при отправке запроса:", err)
			return
		}

		s.Close()
	}
}

// [ABONENT]
func (a *Agent) prepareForElection(segmentPeers []AgentPeerInfoPeer) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(a.node.Host.ID().String())
	config.Logger = hclog.New(&hclog.LoggerOptions{
		Name:  "raft",
		Level: hclog.Error,
	})

	store := raft.NewInmemStore()
	logStore := raft.NewInmemStore()
	snapshotStore := raft.NewDiscardSnapshotStore()

	transport, err := raftnet.NewLibp2pTransport(a.node.Host, 10*time.Second)
	if err != nil {
		log.Printf("Возникла ошибка при подготовке transport для выборов нового хаба: %v", err)
		return
	}

	raftNode, err := raft.NewRaft(config, &hubelection.HubElectionRaftFSM{}, logStore, store, snapshotStore, transport)
	if err != nil {
		log.Printf("Возникла ошибка при подготовке raftNode для выборов нового хаба: %v", err)
		return
	}

	var servers []raft.Server
	for _, segmentPeer := range segmentPeers {
		if len(segmentPeer.Addrs) > 0 {
			servers = append(servers, raft.Server{
				ID:      raft.ServerID(segmentPeer.ID.String()),
				Address: raft.ServerAddress(segmentPeer.Addrs[0]),
			})
		}
	}

	cfg := raft.Configuration{
		Servers: servers,
	}

	raftNode.BootstrapCluster(cfg)

	observerlCn := make(chan raft.Observation, 1)
	obs := raft.NewObserver(
		observerlCn,
		false,
		func(o *raft.Observation) bool {
			_, ok := o.Data.(raft.LeaderObservation)
			return ok
		},
	)

	raftNode.RegisterObserver(obs)
	defer raftNode.DeregisterObserver(obs)

	for {
		select {
		case obsEvent := <-observerlCn:
			if leaderObs, ok := obsEvent.Data.(raft.LeaderObservation); ok {
				if leaderObs.LeaderID == raft.ServerID(a.node.Host.ID().String()) {
					fmt.Println("👑 Я выбран хабом")
					a.fsm.Event(fsm.BecameHubAfterElectionFSMEvent)
				} else {
					leaderPeerID, err := peer.Decode(string(leaderObs.LeaderID))
					if err != nil {
						log.Println("❌ Не удалось декодировать LeaderID в peer.ID:", leaderObs.LeaderID, err)
					} else {
						leaderAddrs := a.node.Host.Peerstore().Addrs(leaderPeerID)
						fmt.Println("👑 Я не выбран хабом. Теперь мой хаб он:", leaderAddrs, leaderPeerID)
						a.fsm.Event(fsm.BecameAbonentAfterElectionFSMEvent, leaderAddrs[0].String(), leaderPeerID.String())
					}
				}

				time.Sleep(5 * time.Second)
				raftNode.Shutdown()

				return
			}
		}
	}
}

// [ABONENT]
func (a *Agent) initializeElectionForMySegment(segmentPeers []AgentPeerInfoPeer) {
	segmentPeerInfos := make([]messages.InfoAboutSegmentPeerInfo, 0)

	for _, peerInfo := range segmentPeers {
		addrs := a.node.PeerAddrs(peerInfo.ID)

		segmentPeerInfos = append(segmentPeerInfos, messages.InfoAboutSegmentPeerInfo{
			ID:    peerInfo.ID,
			IsHub: peerInfo.IsHub,
			Addrs: addrs,
		})
	}

	body := messages.ElectionRequestMessageBody{
		Peers: segmentPeerInfos,
	}

	if marshaledBody, err := json.Marshal(body); err != nil {
		log.Println("Ошибка при маршалинге тела информации о себе:", err)
		return
	} else {
		infoAboutSegmentMessage := messages.Message{
			Type: messages.ElectionRequestMessageType,
			Body: marshaledBody,
		}

		if marshalledMessage, err := json.Marshal(infoAboutSegmentMessage); err != nil {
			log.Printf("Ошибка при маршаллинге сообщения: %v\n", err)
		} else {
			for _, p := range segmentPeers {
				if p.ID == a.node.Host.ID() {
					continue
				}

				stream, err := a.node.Host.NewStream(context.Background(), p.ID, ProtocolID)
				if err != nil {
					log.Println(err)
					return
				}

				if _, err := stream.Write(append(marshalledMessage, '\n')); err != nil {
					log.Println("Ошибка при отправке запроса:", err)
					return
				}

				stream.Close()
			}
		}
	}

	a.prepareForElection(segmentPeers)
}
