package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"pkg/threats"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	looplabFSM "github.com/looplab/fsm"
	"github.com/multiformats/go-multiaddr"
	"github.com/ruslanonly/agent/config"
	"github.com/ruslanonly/agent/internal/agent/model"
	statusmodel "github.com/ruslanonly/agent/internal/agent/model/status"
	"github.com/ruslanonly/agent/internal/agent/protocols/defaultproto"
	defaultprotomessages "github.com/ruslanonly/agent/internal/agent/protocols/defaultproto/messages"
	"github.com/ruslanonly/agent/internal/agent/protocols/pendinghubproto"
	threatsstorage "github.com/ruslanonly/agent/internal/agent/threats"
	"github.com/ruslanonly/agent/internal/fsm"
	"github.com/ruslanonly/agent/internal/network"
)

type Agent struct {
	node       *network.LibP2PNode
	ctx        context.Context
	fsm        *fsm.AgentFSM
	threatsIPC *threats.ThreatsIPCClient

	// Подключенные абоненты и хабы
	peers      map[peer.ID]model.AgentPeerInfo
	peersMutex sync.RWMutex

	handlingMessageMutex sync.RWMutex

	// Максимальное количество подключенных абонентов и хабов
	peersLimit int

	threatsStorage *threatsstorage.ThreatsStorage
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

	pipeName := threats.Pipename()
	threatsIPC, err := threats.NewThreatsIPCClient(pipeName)
	if err != nil {
		log.Fatalf("Возникла ошибка при инициализации агента: %v", err)
	}

	agent := &Agent{
		node:       libp2pNode,
		threatsIPC: threatsIPC,

		ctx: ctx,

		peers:      make(map[peer.ID]model.AgentPeerInfo),
		peersLimit: peersLimit,
		peersMutex: sync.RWMutex{},
	}

	return agent, nil
}

func (a *Agent) getSplittedPeers() (map[peer.ID]model.AgentPeerInfo, map[peer.ID]model.AgentPeerInfo) {
	hubs := make(map[peer.ID]model.AgentPeerInfo)
	abonents := make(map[peer.ID]model.AgentPeerInfo)

	for peerID, peerInfo := range a.peers {
		if peerInfo.Status.IsAbonent() {
			abonents[peerID] = peerInfo
		} else {
			hubs[peerID] = peerInfo
		}
	}

	return hubs, abonents
}

// [ABONENT]
func (a *Agent) getMyHub() (*model.AgentPeerInfo, bool) {
	for _, peerInfo := range a.peers {
		if peerInfo.Status.IsHub() {
			return &peerInfo, true
		}
	}

	return nil, false
}

// [ABONENT]
func (a *Agent) getPeerPeers(targetPeerID peer.ID) map[peer.ID]model.AgentPeerInfoPeer {
	segmentPeers := make(map[peer.ID]model.AgentPeerInfoPeer)

	for peerID, peerInfo := range a.peers[targetPeerID].Peers {
		if !peerInfo.Status.IsHub() {
			segmentPeers[peerID] = peerInfo
		}
	}

	return segmentPeers
}

// [ABONENT]
func (a *Agent) getSegmentPeers() map[peer.ID]model.AgentPeerInfoPeer {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	myHub, myHubIsFound := a.getMyHub()
	segmentPeers := make(map[peer.ID]model.AgentPeerInfoPeer)

	if myHubIsFound {
		for peerID, peerInfo := range a.peers[myHub.ID].Peers {
			if !peerInfo.Status.IsHub() {
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

func (a *Agent) getHubSlotsStatus() statusmodel.HubSlotsStatus {
	_, abonents := a.getSplittedPeers()

	hasAbonents := len(abonents) != 0
	var status statusmodel.HubSlotsStatus

	if !a.isPeersLimitExceeded() {
		status = statusmodel.FreeHubSlotsStatus
	} else if hasAbonents {
		status = statusmodel.FullHavingAbonentsHubSlotsStatus
	} else {
		status = statusmodel.FullNotHavingAbonentsHubSlotsStatus
	}

	return status
}

func (a *Agent) disconnectPeer(peerID peer.ID, notify bool) {
	fmt.Printf("❌ Отключение от пира: %s\n", peerID)

	if notify {
		s, err := a.node.Host.NewStream(context.Background(), peerID, defaultproto.ProtocolID)
		if err != nil {
			log.Printf("Ошибка при отправке уведомления об отключении: %v\n", err)
			return
		}

		msg := defaultprotomessages.Message{
			Type: defaultprotomessages.DisconnectMessageType,
		}

		if err := json.NewEncoder(s).Encode(msg); err != nil {
			log.Printf("Ошибка при отправке уведомления об отключении: %v\n", err)
			return
		}

		s.Close()
	}

	for _, conn := range a.node.Host.Network().ConnsToPeer(peerID) {
		_ = conn.Close()
	}
	a.node.Host.Peerstore().RemovePeer(peerID)
	delete(a.peers, peerID)
}

func (a *Agent) disconnectAllPeers() {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	peerIDs := make([]peer.ID, 0)
	for pid := range a.peers {
		peerIDs = append(peerIDs, pid)
	}

	log.Printf("Отключение от пиров: %v", peerIDs)
	for _, pid := range peerIDs {
		a.disconnectPeer(pid, true)
	}
}

func (a *Agent) Start(options *StartOptions) {
	a.threatsStorage = threatsstorage.NewThreatsStorage(func(blockedIP net.IP) {
		a.threatsIPC.BlockHostMessage(blockedIP)
		a.broadcastBlockTrafficToAbonents(blockedIP)
	})

	a.node.PrintHostInfo()

	a.startStream()
	a.startHeartbeatStream()

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
			},
			fsm.EnterStateFSMCallbackName(fsm.ListeningMessagesAsHubAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				a.fsm.IAmHub()

				if e.Src == fsm.IdleAgentFSMState ||
					e.Src == fsm.ElectingNewHubAgentFSMState ||
					e.Src == fsm.ListeningMessagesAsAbonentAgentFSMState {
					a.startThreatsStream()
					a.startHubStream()
				}

				_, found := e.FSM.Metadata("infoAboutMeCtx")
				if found {
					return
				}

				infoAboutMeCtx, infoAboutMeCancelCtx := context.WithCancel(context.Background())

				e.FSM.SetMetadata("infoAboutMeCtx", infoAboutMeCtx)
				e.FSM.SetMetadata("infoAboutMeCancelCtx", infoAboutMeCancelCtx)

				go func(ctx context.Context) {
					ticker := time.NewTicker(config.BroadcastingInterval)
					defer ticker.Stop()

					a.InfoAboutMeHubMessage()
					a.broadcastToSegmentInfoAboutSegment()

					for {
						select {
						case <-ctx.Done():
							log.Println("🛑 Цикл оповещения остановлен (отменён через cancel)")
							return
						case <-ticker.C:
							a.InfoAboutMeHubMessage()
							a.broadcastToSegmentInfoAboutSegment()
						}
					}
				}(infoAboutMeCtx)
			},
			fsm.LeaveStateFSMCallbackName(fsm.ListeningMessagesAsHubAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {

			},
			fsm.EnterStateFSMCallbackName(fsm.ListeningMessagesAsAbonentAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				a.fsm.IAmAbonent()

				if e.Src == fsm.ConnectingToHubAgentFSMState {
					a.startThreatsStream()
				}
			},
			fsm.LeaveStateFSMCallbackName(fsm.ListeningMessagesAsAbonentAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				if e.Event == fsm.NotConnectedToHubAgentFSMEvent ||
					e.Event == fsm.ElectNewHubRequestFSMEvent {
					a.closeThreatsStream()
				}
			},
			fsm.EnterStateFSMCallbackName(fsm.OrganizingSegmentHubElectionAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				peerIDs := a.organizeSegmentHubElection()
				fmt.Printf("❇️ ОРГАНИЗОВАНЫ ВЫБОРЫ ДЛЯ %v\n", peerIDs)

				if len(peerIDs) == 0 {
					e.FSM.Event(e_, fsm.OrganizingSegmentHubElectionIsCompletedAgentFSMEvent)
					return
				}

				a.fsm.SetElectionPeers(peerIDs)
			},
			fsm.LeaveStateFSMCallbackName(fsm.OrganizingSegmentHubElectionAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				a.fsm.DeleteElectionPeers()
				fmt.Println("❇️ ВЫБОРЫ ЗАВЕРШИЛИСЬ")

			},
			fsm.EnterStateFSMCallbackName(fsm.ElectingNewHubAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				segmentPeers, ok := e.Args[0].([]model.AgentPeerInfoPeer)

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
			fsm.EnterStateFSMCallbackName(fsm.PendingNewHubAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				notConnectedAndShouldWait := len(e.Args) == 0
				if notConnectedAndShouldWait {
					a.startPendingHubStream()
				} else {
					if shouldSleep, ok := e.Args[2].(bool); !ok || shouldSleep {
						time.Sleep(config.ReconnectTimeout)
					}

					e.FSM.Event(e_, fsm.RequestConnectionFromAbonentToHubAgentFSMEvent, e.Args[0], e.Args[1])
				}
			},
			fsm.LeaveStateFSMCallbackName(fsm.PendingNewHubAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				if slices.Contains(a.node.Host.Mux().Protocols(), pendinghubproto.ProtocolID) {
					a.closePendingHubStream()
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

	go a.threatsIPC.Listen(a.RedTrafficIPCHandler, a.YellowTrafficIPCHandler)
	go func(ctx context.Context) {
		ticker := time.NewTicker(config.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("🛑 Цикл оповещения остановлен (отменён через cancel)")
				return
			case <-ticker.C:
				a.checkAllPeersHeartbeat()
			}
		}
	}(a.ctx)

	<-a.ctx.Done()
	fmt.Println("Агент выключается...")
	_ = a.node.Close()
}

func (a *Agent) bootstrap(addr, peerID string) {
	a.disconnectAllPeers()

	period := config.ReconnectTimeout

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
			s, err := a.node.Host.NewStream(context.Background(), hubAddrInfo.ID, defaultproto.ProtocolID)
			if err != nil {
				log.Println(err)
				return
			}

			msg := defaultprotomessages.Message{
				Type: defaultprotomessages.ConnectRequestMessageType,
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

			var message defaultprotomessages.Message
			if err := json.Unmarshal([]byte(responseRaw), &message); err != nil {
				log.Println("Ошибка при парсинге сообщения:", err)
				return
			}

			if message.Type == defaultprotomessages.ConnectedMessageType {
				log.Printf("Я подключен к хабу")
				a.fsm.Event(fsm.ConnectedToHubAgentFSMEvent)

				var body defaultprotomessages.ConnectedMessageBody
				if err := json.Unmarshal(message.Body, &body); err != nil {
					log.Println("Ошибка при парсинге ответа:", err)
					return
				}

				a.peers[hubAddrInfo.ID] = model.AgentPeerInfo{
					ID:     hubAddrInfo.ID,
					Status: statusmodel.HubFreeP2PStatus, // TODO: Необходимо указывать, что это просто хаб
					Peers:  make(map[peer.ID]model.AgentPeerInfoPeer, 0),
				}

				a.handleInfoAboutSegment(hubAddrInfo.ID, body.Peers)
			} else if message.Type == defaultprotomessages.NotConnectedAndWaitMessageType {
				// Если узел получил такое сообщение, ему необходимо ждать
				log.Print("Я не подключен, но ожидаю сообщения о новом хабе")
				a.fsm.Event(fsm.NotConnectedToHubAgentFSMEvent)
			} else if message.Type == defaultprotomessages.NotConnectedMessageType {
				// Если узел получил такое сообщение, ему необходимо подключиться к тому узлу, который он получил в body
				// А если body пустое, необходимо пытаться подключаться к тому же узлу, к которому подключался

				log.Printf("Я не подключен")

				var body defaultprotomessages.NotConnectedMessageBody
				if err := json.Unmarshal(message.Body, &body); err != nil {
					log.Println("Ошибка при парсинге ответа при неуспешном подключении:", err)
					a.fsm.Event(fsm.NotConnectedToHubAgentFSMEvent, addr, peerID, true)

					return
				} else {
					if body.ID == "" || len(body.Addrs) == 0 {
						a.fsm.Event(fsm.NotConnectedToHubAgentFSMEvent, addr, peerID, true)
					} else {
						a.fsm.Event(fsm.NotConnectedToHubAgentFSMEvent, body.Addrs[0], body.ID.String(), false)
					}
				}
			}

			break
		}
	}
}

// func (a *Agent) connectToHubAsHub(addr multiaddr.Multiaddr, peerID peer.ID) {
// 	period := config.ReconnectTimeout

// 	addrWithPeerID := fmt.Sprintf("%s/p2p/%s", addr, peerID)
// 	maddr, err := multiaddr.NewMultiaddr(addrWithPeerID)
// 	if err != nil {
// 		log.Fatalf("Ошибка парсинга адреса bootstrap: %v", err)
// 	}

// 	log.Printf("Попытка подключиться к хаб-узлу: %s", maddr.String())

// 	for {
// 		hubAddrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
// 		if err != nil {
// 			log.Printf("Ошибка парсинга peer.AddrInfo: %v", err)
// 			time.Sleep(period)
// 			continue
// 		}

// 		if err := a.node.Connect(*hubAddrInfo); err != nil {
// 			log.Printf("Подключение к bootstrap не удалось: %v. Повтор через %s...", err, period)
// 		} else {
// 			s, err := a.node.Host.NewStream(context.Background(), hubAddrInfo.ID, defaultproto.ProtocolID)
// 			if err != nil {
// 				log.Println(err)
// 				return
// 			}

// 			msg := defaultprotomessages.Message{
// 				Type: defaultprotomessages.ConnectRequestMessageAsHubType,
// 			}

// 			if err := json.NewEncoder(s).Encode(msg); err != nil {
// 				log.Println("Ошибка при отправке запрос на подключение:", err)
// 				return
// 			}

// 			reader := bufio.NewReader(s)
// 			responseRaw, err := reader.ReadString('\n')
// 			if err != nil {
// 				log.Println("Ошибка при чтении ответа:", err)
// 				return
// 			}

// 			var message defaultprotomessages.Message
// 			if err := json.Unmarshal([]byte(responseRaw), &message); err != nil {
// 				log.Println("Ошибка при парсинге сообщения:", err)
// 				return
// 			}

// 			if message.Type == defaultprotomessages.ConnectedMessageType {
// 				log.Printf("Я подключен к хабу")
// 				a.fsm.Event(fsm.ConnectedToHubAgentFSMEvent)

// 				var body defaultprotomessages.ConnectedMessageBody
// 				if err := json.Unmarshal(message.Body, &body); err != nil {
// 					log.Println("Ошибка при парсинге ответа:", err)
// 					return
// 				}

// 				a.peers[hubAddrInfo.ID] = model.AgentPeerInfo{
// 					ID:     hubAddrInfo.ID,
// 					Status: statusmodel.HubFreeP2PStatus, // TODO: Необходимо указывать, что это просто хаб
// 					Peers:  make(map[peer.ID]model.AgentPeerInfoPeer, 0),
// 				}

// 				a.handleInfoAboutSegment(hubAddrInfo.ID, body.Peers)
// 			}
// 			break
// 		}
// 	}
// }

func (a *Agent) startStream() {
	log.Println("Установлен обработчик сообщений для hub-потока")

	a.node.SetStreamHandler(defaultproto.ProtocolID, a.streamHandler)

	a.node.Host.Network().Notify(a.node.Host.ConnManager().Notifee())

}

func (a *Agent) streamHandler(stream libp2pNetwork.Stream) {
	defer stream.Close()

	a.handlingMessageMutex.Lock()
	defer a.handlingMessageMutex.Unlock()

	decoder := json.NewDecoder(stream)
	var msg defaultprotomessages.Message

	if err := decoder.Decode(&msg); err != nil {
		log.Printf(
			"Ошибка при обработке потока сообщений от (%s %s): %v",
			stream.Conn().RemotePeer(),
			stream.Conn().RemoteMultiaddr(),
			err,
		)
		stream.Close()
		return
	}

	log.Printf("Получено сообщение: %s %s", msg.Type, string(msg.Body))

	if msg.Type == defaultprotomessages.ConnectRequestMessageType {
		a.handleConnectionRequestMessage(stream)
	} else if msg.Type == defaultprotomessages.BecomeOnlyOneHubMessageType {
		fmt.Println("[BecomeOnlyOneHubMessageType] Я получил сообщение о том, что необходимо стать хабом")
		err := a.fsm.Event(fsm.BecomeHubAgentFSMEvent)
		if err != nil {
			fmt.Println("Я не могу стать хабом: ", err)

			message := defaultprotomessages.Message{
				Type: defaultprotomessages.ICantBecomeOnlyOneHubMessageType,
			}
			if err := json.NewEncoder(stream).Encode(message); err != nil {
				log.Println("Ошибка при отправке запроса:", err)
			}
		} else {
			message := defaultprotomessages.Message{
				Type: defaultprotomessages.IBecameOnlyOneHubMessageType,
			}
			if err := json.NewEncoder(stream).Encode(message); err != nil {
				log.Println("Ошибка при отправке запроса:", err)
			}
		}
	} else if msg.Type == defaultprotomessages.InitializeElectionRequestMessageType {
		var body defaultprotomessages.InitializeElectionRequestMessageBody
		if err := json.Unmarshal([]byte(msg.Body), &body); err != nil {
			log.Printf("Ошибка при парсинге сообщения: %v", err)
			return
		}

		a.handleInfoAboutSegment(stream.Conn().RemotePeer(), body.Peers)

		segmentPeersArr := make([]model.AgentPeerInfoPeer, 0)
		for _, p := range body.Peers {
			agentPeerInfoPeer := model.AgentPeerInfoPeer{
				ID:     p.ID,
				Addrs:  p.Addrs,
				Status: statusmodel.AbonentP2PStatus,
			}
			segmentPeersArr = append(segmentPeersArr, agentPeerInfoPeer)
		}

		log.Println("🚩 Я должен начать выборы")
		err := a.fsm.Event(fsm.ElectNewHubRequestFSMEvent, segmentPeersArr, true)
		if err != nil {
			log.Printf("Ошибка при FSM переходе: %v", err)
		}
	} else if msg.Type == defaultprotomessages.InfoAboutSegmentMessageType {
		var infoAboutSegment defaultprotomessages.InfoAboutSegmentMessageBody
		if err := json.Unmarshal([]byte(msg.Body), &infoAboutSegment); err != nil {
			log.Println("Ошибка при парсинге ответа:", err)
			return
		}

		a.handleInfoAboutSegment(stream.Conn().RemotePeer(), infoAboutSegment.Peers)
	} else if msg.Type == defaultprotomessages.ElectionRequestMessageType {
		log.Println("🚩 Меня позвали участвовать в выборах нового хаба")

		var infoAboutSegment defaultprotomessages.InfoAboutSegmentMessageBody
		if err := json.Unmarshal([]byte(msg.Body), &infoAboutSegment); err != nil {
			log.Println("Ошибка при парсинге ответа:", err)
			return
		}

		// TODO: COPY PASTE FROM msg.Type == defaultprotomessages.InfoAboutSegmentMessageType
		segmentPeersMap := a.getSegmentPeers()
		segmentPeersArr := make([]model.AgentPeerInfoPeer, 0)
		for _, p := range segmentPeersMap {
			segmentPeersArr = append(segmentPeersArr, p)
		}

		err := a.fsm.Event(fsm.ElectNewHubRequestFSMEvent, segmentPeersArr, false)
		if err != nil {
			log.Printf("Ошибка при FSM переходе: %v", err)
		}
	} else if msg.Type == defaultprotomessages.DisconnectMessageType {
		a.disconnectPeer(stream.Conn().RemotePeer(), false)
	}
}

// [HUB] Обработка запроса на подключение
func (a *Agent) handleConnectionRequestMessage(stream libp2pNetwork.Stream) {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	slotsStatus := a.getHubSlotsStatus()

	log.Printf("🔱 Мой статус: %s", slotsStatus)
	if slotsStatus == statusmodel.FreeHubSlotsStatus {
		a.handleConnectedOnConnectionRequest(stream)
	} else {
		var msg defaultprotomessages.Message

		if slotsStatus == statusmodel.FullHavingAbonentsHubSlotsStatus && a.fsm.FSM.Can(fsm.OrganizeSegmentHubElectionAgentFSMEvent) {
			msg = defaultprotomessages.Message{
				Type: defaultprotomessages.NotConnectedAndWaitMessageType,
			}

			addrInfo := peer.AddrInfo{
				ID:    stream.Conn().RemotePeer(),
				Addrs: []multiaddr.Multiaddr{stream.Conn().RemoteMultiaddr()},
			}

			a.fsm.AddPendingHubPeer(addrInfo)
			if a.fsm.FSM.Can(fsm.OrganizeSegmentHubElectionAgentFSMEvent) {
				fmt.Println("👍 Can Organize Hub Elections")
				a.fsm.Event(fsm.OrganizeSegmentHubElectionAgentFSMEvent)
			} else {
				fmt.Println("👎 Can't Organize Hub Elections")
			}
		} else {
			var body *defaultprotomessages.NotConnectedMessageBody = nil
			foundFreeHub := false

			knownHubs := a.fsm.GetKnownHubs()

			hubs, _ := a.getSplittedPeers()
			for _, hub := range hubs {
				addrs := a.node.PeerAddrs(hub.ID)

				if hub.Status == statusmodel.HubFreeP2PStatus {
					body = &defaultprotomessages.NotConnectedMessageBody{
						ID:    hub.ID,
						Addrs: addrs,
					}
					foundFreeHub = true
					break
				}
			}

			if !foundFreeHub {
				for _, hub := range knownHubs {
					if hub.Status == statusmodel.HubFreeP2PStatus {
						body = &defaultprotomessages.NotConnectedMessageBody{
							ID:    hub.ID,
							Addrs: hub.Addrs,
						}
						foundFreeHub = true
						break
					}
				}
			}

			if !foundFreeHub {
				foundFullHub := false

				for _, hub := range hubs {
					addrs := a.node.PeerAddrs(hub.ID)
					if hub.Status == statusmodel.HubFullHavingAbonentsP2PStatus {
						body = &defaultprotomessages.NotConnectedMessageBody{
							ID:    hub.ID,
							Addrs: addrs,
						}

						foundFullHub = true
						break
					}
				}

				if !foundFullHub {
					for _, hub := range knownHubs {
						if hub.Status == statusmodel.HubFullHavingAbonentsP2PStatus {
							body = &defaultprotomessages.NotConnectedMessageBody{
								ID:    hub.ID,
								Addrs: hub.Addrs,
							}
							break
						}
					}
				}
			}

			msg = defaultprotomessages.Message{
				Type: defaultprotomessages.NotConnectedMessageType,
			}

			if body != nil {
				if marshalledBody, err := json.Marshal(*body); err != nil {
					log.Println("Ошибка при маршалинге информации о свободных хабах для подключения:", err)
				} else {
					fmt.Printf("❤️‍🔥 Подключайся к этому хабу: %s\n", body.ID)
					msg = defaultprotomessages.Message{
						Type: defaultprotomessages.NotConnectedMessageType,
						Body: marshalledBody,
					}
				}
			}
		}

		if err := json.NewEncoder(stream).Encode(msg); err != nil {
			log.Printf("Ошибка при отправке сообщения об успешном подключении узла %s: %v\n", stream.Conn().RemotePeer(), err)
			return
		}
	}
}

// [HUB]
func (a *Agent) handleConnectedOnConnectionRequest(stream libp2pNetwork.Stream) {
	remotePeerID := stream.Conn().RemotePeer()

	_, abonents := a.getSplittedPeers()
	abonentsPeerInfos := make([]defaultprotomessages.InfoAboutSegmentPeerInfo, 0)

	for peerID, peerInfo := range abonents {
		addrs := a.node.PeerAddrs(peerID)

		abonentsPeerInfos = append(abonentsPeerInfos, defaultprotomessages.InfoAboutSegmentPeerInfo{
			ID:    peerID,
			IsHub: peerInfo.Status.IsHub(),
			Addrs: addrs,
		})
	}

	body := defaultprotomessages.ConnectedMessageBody{
		Peers: abonentsPeerInfos,
	}

	if marshaledBody, err := json.Marshal(body); err != nil {
		log.Println("Ошибка при маршалинге тела информации о себе:", err)
		return
	} else {
		infoAboutSegmentMessage := defaultprotomessages.Message{
			Type: defaultprotomessages.ConnectedMessageType,
			Body: marshaledBody,
		}

		if err := json.NewEncoder(stream).Encode(infoAboutSegmentMessage); err != nil {
			log.Printf("Ошибка при отправке сообщения об успешном подключении узла %s: %v\n", remotePeerID, err)
			return
		}
	}

	log.Printf("Подключен новый узел %s\n", remotePeerID)

	a.peers[remotePeerID] = model.AgentPeerInfo{
		ID:     remotePeerID,
		Status: statusmodel.AbonentP2PStatus,
		Peers:  nil,
	}
}

// [HUB]
func (a *Agent) broadcastToSegmentInfoAboutSegment() {
	a.peersMutex.RLock()
	defer a.peersMutex.RUnlock()

	_, abonents := a.getSplittedPeers()
	abonentsPeerIDs := make([]peer.ID, 0)
	abonentsPeerInfos := make([]defaultprotomessages.InfoAboutSegmentPeerInfo, 0)

	for peerID, peerInfo := range abonents {
		addrs := a.node.PeerAddrs(peerID)
		abonentsPeerIDs = append(abonentsPeerIDs, peerID)

		abonentsPeerInfos = append(abonentsPeerInfos, defaultprotomessages.InfoAboutSegmentPeerInfo{
			ID:    peerID,
			IsHub: peerInfo.Status.IsHub(),
			Addrs: addrs,
		})
	}

	if len(abonentsPeerIDs) == 0 {
		return
	}

	infoAboutSegment := defaultprotomessages.InfoAboutSegmentMessageBody{
		Peers: abonentsPeerInfos,
	}

	if marshaledBody, err := json.Marshal(infoAboutSegment); err != nil {
		log.Println("Ошибка при маршалинге тела информации о себе:", err)
		return
	} else {

		infoAboutSegmentMessage := defaultprotomessages.Message{
			Type: defaultprotomessages.InfoAboutSegmentMessageType,
			Body: marshaledBody,
		}

		if marshaledMessage, err := json.Marshal(infoAboutSegmentMessage); err != nil {
			log.Println("Ошибка при маршалинге информации о сегменте:", err)
			return
		} else {
			log.Printf("Отправка broadcast-сообщение о сегменте %v", abonentsPeerIDs)
			a.node.BroadcastToPeers(defaultproto.ProtocolID, abonentsPeerIDs, marshaledMessage)
		}
	}
}

// [ABONENT]
func (a *Agent) handleInfoAboutSegment(hubID peer.ID, peers []defaultprotomessages.InfoAboutSegmentPeerInfo) {
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

		var status statusmodel.PeerP2PStatus
		if p.IsHub {
			status = statusmodel.HubFreeP2PStatus
		} else {
			status = statusmodel.AbonentP2PStatus
		}

		connectedness := a.node.Host.Network().Connectedness(p.ID)

		if connectedness == libp2pNetwork.Connected || p.ID == a.node.Host.ID() {
			a.peers[hubID].Peers[p.ID] = model.AgentPeerInfoPeer{
				ID:     p.ID,
				Addrs:  p.Addrs,
				Status: status,
			}
		} else {
			info := peer.AddrInfo{
				ID:    p.ID,
				Addrs: mas,
			}

			if err := a.node.Connect(info); err != nil {
				log.Printf("Возникла ошибка при подключении пира %s: %v", p.ID, err)
			} else {
				a.peers[hubID].Peers[p.ID] = model.AgentPeerInfoPeer{
					ID:     p.ID,
					Addrs:  p.Addrs,
					Status: status,
				}
			}
		}
	}
}
