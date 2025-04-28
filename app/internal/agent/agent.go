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

	a.peersMutex.RLock()
	for peerID, peerInfo := range a.peers {
		if peerInfo.IsHub {
			hubs[peerID] = peerInfo
		} else {
			abonents[peerID] = peerInfo
		}
	}
	a.peersMutex.RUnlock()

	return hubs, abonents
}

func (a *Agent) isPeersLimitExceeded() bool {
	a.peersMutex.RLock()
	out := len(a.peers) >= a.peersLimit
	a.peersMutex.RUnlock()

	return out
}

func (a *Agent) getHubSlotsStatus() messages.HubSlotsStatus {
	a.peersMutex.RLock()
	defer a.peersMutex.RUnlock()
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

func (a *Agent) Start(options *StartOptions) {
	a.node.PrintHostInfo()

	a.fsm = fsm.NewAgentFSM(
		a.ctx,
		looplabFSM.Callbacks{
			"enter_state": func(e_ context.Context, e *looplabFSM.Event) {
				log.Printf("📦 FSM переход: %s -> %s по событию '%s' с аргументами %s", e.Src, e.Dst, e.Event, e.Args)
			},
			fsm.ConnectingToHubAgentFSMState: func(e_ context.Context, e *looplabFSM.Event) {
				bootstrapIP, ok1 := e.Args[0].(string)
				bootstrapPeerID, ok2 := e.Args[1].(string)
				if !ok1 || !ok2 {
					log.Println("❌ Неверные аргументы для ReadInitialSettingsAgentFSMEvent")
					return
				}

				a.bootstrap(bootstrapIP, bootstrapPeerID)
				e.FSM.SetMetadata(fsm.RoleAgentFSMMetadataKey, fsm.AbonentRole)
			},
			fsm.EnterStateFSMCallbackName(fsm.ListeningMessagesAsHubAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				e.FSM.SetMetadata(fsm.RoleAgentFSMMetadataKey, fsm.HubRole)
				a.startStream()

				infoAboutMeCtx, infoAboutMeCancelCtx := context.WithCancel(context.Background())

				e.FSM.SetMetadata("infoAboutMeCtx", infoAboutMeCtx)
				e.FSM.SetMetadata("infoAboutMeCancelCtx", infoAboutMeCancelCtx)

				go func(ctx context.Context) {
					ticker := time.NewTicker(10 * time.Second)
					defer ticker.Stop()

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

			},
		},
	)

	if options != nil {
		// Узел начинает свою работу как обычный абонент
		a.fsm.Event(fsm.ReadInitialSettingsAgentFSMEvent, options.BootstrapIP, options.BootstrapPeerID)
	} else {
		// Узел начинает свою работу как хаб
		a.fsm.Event(fsm.BecomeHubAgentFSMEvent)
	}

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

func (a *Agent) bootstrap(ip, peerID string) {
	period := 10 * time.Second

	if ip == "" {
		log.Println("BOOTSTRAP_IP не задан — агент запускается как первый узел (hub?)")
		return
	}

	bootstrapAddr := fmt.Sprintf("/ip4/%s/tcp/5000/p2p/%s", ip, peerID)
	maddr, err := multiaddr.NewMultiaddr(bootstrapAddr)
	if err != nil {
		log.Fatalf("Ошибка парсинга адреса bootstrap: %v", err)
	}

	log.Printf("Попытка подключиться к bootstrap-узлу: %s", maddr.String())

	for {
		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			log.Printf("Ошибка парсинга peer.AddrInfo: %v", err)
			time.Sleep(period)
			continue
		}

		if err := a.node.Connect(*info); err != nil {
			log.Printf("Подключение к bootstrap не удалось: %v. Повтор через %s...", err, period)
			if strings.Contains(err.Error(), "peer id mismatch") {
				re := regexp.MustCompile(`remote key matches ([\w\d]+)`)
				matches := re.FindStringSubmatch(err.Error())
				if len(matches) > 1 {
					actualBootstrapPeerID := matches[1]
					log.Printf("⚠️ Обнаружен актуальный PeerID: %s", actualBootstrapPeerID)
					a.bootstrap(ip, actualBootstrapPeerID)
					break
				}
			}
			time.Sleep(period)
		} else {
			s, err := a.node.Host.NewStream(context.Background(), info.ID, ProtocolID)
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
				log.Println("Ошибка при парсинге ответа:", err)
				return
			}

			if message.Type == messages.ConnectedMessageType {
				log.Print("Я подключен к хабу")
				a.peersMutex.Lock()
				a.peers[info.ID] = AgentPeerInfo{
					ID:    info.ID,
					IsHub: true,
					Peers: make(map[peer.ID]AgentPeerInfoPeer, 0),
				}
				a.peersMutex.Unlock()
				a.fsm.Event(fsm.ConnectedToHubAgentFSMEvent)
			} else if message.Type == messages.NotConnectedMessageType {
				log.Print("Узел не подключен")
				a.fsm.Event(fsm.NotConnectedToHubAgentFSMEvent, ip, peerID)
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
		err := a.fsm.Event(fsm.ElectNewHubRequestFSMEvent)
		if err != nil {
			log.Printf("Ошибка при FSM переходе: %v", err)
		}
	} else if msg.Type == messages.InfoAboutSegmentMessageType {
		a.handleInfoAboutSegment(stream.Conn().RemotePeer(), msg)
	}
}

// [HUB] Обработка запроса на подключение
func (a *Agent) handleConnectionRequestMessage(stream libp2pNetwork.Stream) {
	remotePeerID := stream.Conn().RemotePeer()

	slotsStatus := a.getHubSlotsStatus()
	var msg messages.Message
	if slotsStatus == messages.FreeHubSlotsStatus {
		a.peersMutex.Lock()
		a.peers[remotePeerID] = AgentPeerInfo{
			ID:    remotePeerID,
			IsHub: false,
			Peers: nil,
		}
		count := len(a.peers)
		a.peersMutex.Unlock()
		log.Print("Подключен", remotePeerID, slotsStatus, count, a.peersLimit)

		msg = messages.Message{
			Type: messages.ConnectedMessageType,
		}
	} else if slotsStatus == messages.FullHavingAbonentsHubSlotsStatus {
		msg = messages.Message{
			Type: messages.NotConnectedAndWaitMessageType,
		}

		a.fsm.Event(fsm.OrganizeSegmentHubElectionAgentFSMEvent)
	} else {
		msg = messages.Message{
			Type: messages.NotConnectedMessageType,
		}
	}

	if err := json.NewEncoder(stream).Encode(msg); err != nil {
		log.Printf("Ошибка при отправке сообщения об неуспешном подключении узлу %s: %v\n", stream.Conn().RemotePeer(), err)
		return
	}

	stream.Close()
}

// [HUB] Отправление информации о себе хабам
func (a *Agent) broadcastToHubsInfoAboutMe() {
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

// [HUB]
func (a *Agent) broadcastToSegmentInfoAboutSegment() {
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
			log.Printf("Отправка broadcast-сообщение о сегменте", abonentsPeerIDs)
			a.node.BroadcastToPeers(ProtocolID, abonentsPeerIDs, marshaledMessage)
		}
	}
}

// [ABONENT]
func (a *Agent) handleInfoAboutSegment(hubID peer.ID, message messages.Message) {
	var infoAboutSegment messages.InfoAboutSegmentMessageBody
	if err := json.Unmarshal([]byte(message.Body), &infoAboutSegment); err != nil {
		log.Println("Ошибка при парсинге ответа:", err)
		return
	}

	for _, p := range infoAboutSegment.Peers {
		connectedness := a.node.Host.Network().Connectedness(p.ID)

		a.peersMutex.RLock()
		if _, found := a.peers[hubID].Peers[p.ID]; found || connectedness == libp2pNetwork.Connected || p.ID == a.node.Host.ID() || p.ID == hubID {
			a.peersMutex.RUnlock()
			continue
		}
		a.peersMutex.RUnlock()

		mas, err := network.MultiaddrsStrsToMultiaddrs(p.Addrs)

		if err != nil {
			log.Printf("Возникла ошибка при обработке адресов пира %s: %v", p.ID, err)
			continue
		}

		info := peer.AddrInfo{
			ID:    p.ID,
			Addrs: mas,
		}

		if err := a.node.Connect(info); err != nil {
			log.Printf("Возникла ошибка при подключении пира %s: %v", p.ID, err)
		} else {
			a.peersMutex.Lock()
			a.peers[hubID].Peers[p.ID] = AgentPeerInfoPeer{
				ID:    p.ID,
				Addrs: p.Addrs,
				IsHub: p.IsHub,
			}
			a.peersMutex.Unlock()
		}
	}

	log.Printf("Пиры хаба после получения информации о сегменте: %v", a.peers[hubID].Peers)
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
			message = messages.Message{
				Type: messages.InitializeElectionRequestMessageType,
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
func (a *Agent) initializeElectionForMySegment() {
	peerID := a.node.Host.ID()

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(peerID)

	store := raft.NewInmemStore()
	logStore := raft.NewInmemStore()
	snapshotStore := raft.NewDiscardSnapshotStore()

	transport, err := raftnet.NewLibp2pTransport(a.node.Host, 10*time.Second)
	if err != nil {

	}

	raftNode, err := raft.NewRaft(config, &hubelection.HubElectionRaftFSM{}, logStore, store, snapshotStore, transport)
	if err != nil {

	}

	a.fsm.FSM.SetMetadata("RaftNode", raftNode)

	cfg := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(peerID),
				Address: raft.ServerAddress(peerID),
			},
			{
				ID:      raft.ServerID(peerID),
				Address: raft.ServerAddress(peerID),
			},
		},
	}

	raftNode.BootstrapCluster(cfg)

	channel := make(chan raft.Observation, 1)
	obs := raft.NewObserver(
		channel,
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
		case obsEvent := <-channel:
			if leaderObs, ok := obsEvent.Data.(raft.LeaderObservation); ok {
				fmt.Println("👑 Новый лидер выбран:", leaderObs.LeaderAddr, leaderObs.LeaderID)
				return
			}
		case <-time.After(10 * time.Second):
			fmt.Println("⚠️ Таймаут ожидания выбора лидера")
			return
		}
	}
}
