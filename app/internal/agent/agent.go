package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	libp2pNetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	looplabFSM "github.com/looplab/fsm"
	"github.com/multiformats/go-multiaddr"
	"github.com/ruslanonly/p2p/internal/agent/messages"
	"github.com/ruslanonly/p2p/internal/fsm"
	"github.com/ruslanonly/p2p/internal/network"
)

type AgentPeerInfo struct {
	ID peer.ID
	isHub bool
}

type Agent struct {
	node *network.LibP2PNode
	ctx context.Context
	fsm *fsm.AgentFSM
	// Подключенные абоненты и хабы
	peers map[peer.ID]AgentPeerInfo
	// Максимальное количество подключенных абонентов и хабов
	peersLimit int
}

type StartOptions struct {
	BootstrapIP string
	BootstrapPeerID string
}

func NewAgent(ctx context.Context, peersLimit, port int) (*Agent, error) {
	libp2pNode, err := network.NewLibP2PNode(ctx, port)

	if (err != nil) {
		log.Fatalf("Возникла ошибка при инициализации агента: %v", err)
	}

	agent := &Agent{
		node: libp2pNode,
		ctx: ctx,
		peers: make(map[peer.ID]AgentPeerInfo),
		peersLimit: peersLimit,
	}

	return agent, nil
}

func (a *Agent) Start(options *StartOptions) {
	a.node.PrintHostInfo()

	a.fsm = fsm.NewAgentFSM(
		looplabFSM.Callbacks{
			"enter_state": func(e_ context.Context, e *looplabFSM.Event) {
				log.Printf("📦 FSM переход: %s -> %s по событию '%s'", e.Src, e.Dst, e.Event)
			},
			fsm.ReadInitialSettingsAgentFSMEvent: func(e_ context.Context, e *looplabFSM.Event) {
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
			},
			fsm.LeaveStateFSMCallbackName(fsm.ListeningMessagesAsHubAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
			},
			fsm.EnterStateFSMCallbackName(fsm.ListeningMessagesAsAbonentAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
				e.FSM.SetMetadata(fsm.RoleAgentFSMMetadataKey, fsm.AbonentRole)
				a.startStream()
			},
			fsm.LeaveStateFSMCallbackName(fsm.ListeningMessagesAsAbonentAgentFSMState): func(e_ context.Context, e *looplabFSM.Event) {
			},
		},
	)

	if (options != nil) {
		// Узел начинает свою работу как обычный абонент
		a.fsm.FSM.Event(a.ctx, fsm.ReadInitialSettingsAgentFSMEvent, options.BootstrapIP, options.BootstrapPeerID)
	} else {
		// Узел начинает свою работу как хаб
		a.fsm.FSM.Event(a.ctx, fsm.BecomeHubAgentFSMEvent)
	}

	<-a.ctx.Done()
	fmt.Println("Агент выключается...")
	_ = a.node.Close()
}

func (a *Agent) isHub() bool {
	raw, ok := a.fsm.FSM.Metadata(fsm.RoleAgentFSMMetadataKey)
	metadataRole, err := raw.(fsm.RoleAgentFSMMetadataValue)
	if (!ok || err) {
		log.Println("Возникла ошибка при обработке FSM Metadata Role")
	}

	return metadataRole == fsm.HubRole
}

func (a *Agent) isAbonent() bool {
	raw, ok := a.fsm.FSM.Metadata(fsm.RoleAgentFSMMetadataKey)
	metadataRole, err := raw.(fsm.RoleAgentFSMMetadataValue)
	if (!ok || err) {
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

		if err := a.node.Host.Connect(a.ctx, *info); err != nil {
			log.Printf("Подключение к bootstrap не удалось: %v. Повтор через %s...", err, period)
			if strings.Contains(err.Error(), "peer id mismatch") {
				re := regexp.MustCompile(`remote key matches ([\w\d]+)`)
				matches := re.FindStringSubmatch(err.Error())
				if len(matches) > 1 {
					actualBootstrapPeerID := matches[1]
					log.Printf("⚠️ Обнаружен актуальный PeerID: %s", actualBootstrapPeerID)
					a.bootstrap(ip, actualBootstrapPeerID)
					break;
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

			if (message.Type == messages.ConnectedMessageType) {
				log.Print("Узел подключен")
				a.peers[info.ID] = AgentPeerInfo{
					ID: info.ID,
					isHub: true,
				}
				a.fsm.FSM.Event(a.ctx, fsm.ConnectedToHubAgentFSMEvent)
			} else if (message.Type == messages.NotConnectedMessageType) {
				log.Print("Узел не подключен")
				a.fsm.FSM.Event(a.ctx, fsm.NotConnectedToHubAgentFSMEvent, ip, peerID)
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
		log.Fatalf("Ошибка при обработке потока сообщений: %v", err)
	}

	log.Printf("Получено сообщение: %s", raw)

	var msg messages.Message
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		log.Printf("Ошибка при парсинге сообщения: %v", err)
		return
	}

	log.Println(msg)
	log.Println(a.node.Host.Peerstore())

	if (msg.Type == messages.ConnectRequestMessageType) {
		remotePeerID := stream.Conn().RemotePeer()

		var msg messages.Message
		if len(a.peers) >= a.peersLimit {
			msg = messages.Message{
				Type: messages.NotConnectedMessageType,
			}
		} else {
			a.peers[remotePeerID] = AgentPeerInfo{
				ID: remotePeerID,
				isHub: false,
			}
			msg = messages.Message{
				Type: messages.ConnectedMessageType,
			}
		}

		if err := json.NewEncoder(stream).Encode(msg); err != nil {
			log.Printf("Ошибка при отправке сообщения об неуспешном подключении узлу %s: %v\n", stream.Conn().RemotePeer(), err)
			return
		}

		stream.Close()
	}
}
