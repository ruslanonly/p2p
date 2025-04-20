package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ruslanonly/p2p/host/fsm"
)

func sendJSONUDP(addr string, msg Message) {
	conn, err := net.Dial("udp", addr)
	if err != nil {
		log.Printf("⚠️ Ошибка отправки на %s: %v", addr, err)
		return
	}
	defer conn.Close()

	data, _ := json.Marshal(msg)
	conn.Write(append(data, '\n'))
}

func sendJSONTCP(conn net.Conn, msg Message) {
    messageBytes, err := json.Marshal(msg)
    if err != nil {
        log.Printf("❌ Ошибка при сериализации сообщения в JSON: %v", err)
        return
    }

    _, err = conn.Write(append(messageBytes, '\n'))
    if err != nil {
        log.Printf("❌ Ошибка при отправке сообщения через TCP: %v", err)
    }
}

type Agent struct {
	host          string
	maxPeers      int
	peers         map[string]PeerInfo
	peersMutex    sync.Mutex
	waitingPeers  []string
	fsm           *fsm.AgentFSM
}

type AgentConfiguration struct {
	Host string
	MaxPeers int
}

type BootstrapAddress struct {
	IP string
}

func New(configuration AgentConfiguration) *Agent {
	
	return &Agent{
		host:          configuration.Host,
		maxPeers:      configuration.MaxPeers,
		peers:         make(map[string]PeerInfo),
		waitingPeers:  make([]string, 0),
		fsm: fsm.NewAgentFSM(),
	}
}

func (a *Agent) getPeers() map[string]PeerInfo {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	peers := make(map[string]PeerInfo)

	for addr, peerInfo := range a.peers {
		if !peerInfo.IsSuper {
			peers[addr] = peerInfo
		}
	}

	return peers
}

func (a *Agent) getSuperpeers() map[string]PeerInfo {
	a.peersMutex.Lock()
	defer a.peersMutex.Unlock()

	superpeers := make(map[string]PeerInfo)

	for addr, peerInfo := range a.peers {
		if peerInfo.IsSuper {
			superpeers[addr] = peerInfo
		}
	}

	return superpeers
}

func (a *Agent) Start(bootstrapAddress *BootstrapAddress) {
	go a.listenUDP()
	go a.listenTCP()
	a.fsm.Fsm.Event(context.Background(), fsm.ReadInitialSettingsAgentFSMEvent, bootstrapAddress)
}


func (a *Agent) listenTCP() {
	addr := fmt.Sprintf("%s:%d", a.host, 55000)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Ошибка при запуске сервера: %v", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()

		if err != nil {
			log.Printf("Ошибка подключения: %v", err)
			continue
		}
		go a.handleTCPConnection(conn)
	}
}

func (a *Agent) handleTCPConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	messageStr, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Ошибка чтения: %v", err)
		return
	}

	var msg Message
	if err := json.Unmarshal([]byte(messageStr), &msg); err != nil {
		log.Printf("❌ Ошибка при разборе JSON-сообщения: %v", err)
		return
	}

	senderIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		log.Printf("Ошибка обработки адреса удаленного хоста: %v", err)
		return
	}

	log.Printf("📩 Получено сообщение от %s: %+v", senderIP, msg)

	switch msg.Type {
	case ConnectRequestMessageType:
		a.peersMutex.Lock()
		if len(a.peers) < a.maxPeers {
			a.peers[senderIP] = PeerInfo{
				IP:      senderIP,
				IsSuper: false,
			}
			a.peersMutex.Unlock()

			sendJSONTCP(conn, Message{Type: ConnectedMessageType})
			log.Printf("✅ Подключён новый узел: %s", senderIP)

			a.reportPeers([]string{senderIP})
		} else {
			a.peersMutex.Unlock()
			a.waitingPeers = append(a.waitingPeers, senderIP)

			sendJSONTCP(conn, Message{Type: WaitMessageType})

			log.Printf("⏳ Добавлен в очередь ожидания: %s", senderIP)

			superpeers := a.getSuperpeers()
			if len(superpeers) > 0 {
				log.Printf("🔍 Поиск суперпира среди текущих")

				for super := range superpeers {
					sendJSONUDP(super, Message{Type: FindSuperpeerMessageType})
				}
			} else {
				log.Printf("🗳 Инициация выборов нового суперпира")
				peers := a.getPeers()
				for peerIP := range peers {
					addr := &net.UDPAddr{
						IP:   net.ParseIP(peerIP),
						Port: 55001,
					}

					sendJSONUDP(addr.String(), Message{Type: ElectNewSuperpeerMessageType})
				}
			}
		}

	case DiscoverPeersMessageType:
		a.peersMutex.Lock()
		peersList := make(DiscoverPeersResponse, 0, len(a.peers))
		for _, peerInfo := range a.peers {
			peersList = append(peersList, peerInfo)
		}
		a.peersMutex.Unlock()

		body, err := json.Marshal(peersList)
		if err != nil {
			log.Printf("Ошибка маршалинга структуры: %v", err)
			return
		}

		conn.Write(body)
	default:
		log.Printf("⚠️ Неизвестный тип сообщения: %s", msg.Type)
	}
}

func (a *Agent) listenUDP() {
	addr := net.UDPAddr{
		Port: 55001,
		IP:   net.ParseIP(a.host),
	}

	conn, err := net.ListenUDP("udp", &addr)

	if err != nil {
		log.Fatalf("Не удалось запустить UDP сервер: %v", err)
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Ошибка при чтении UDP: %v", err)
			continue
		}

		message := strings.TrimSpace(string(buffer[:n]))
		log.Printf("📨 Получено UDP сообщение от %s: %s", remoteAddr, message)

		go a.handleUDPMessage(message, remoteAddr)
	}
}

func (a *Agent) handleUDPMessage(message string, remoteAddr *net.UDPAddr) {
	var msg Message
	if err := json.Unmarshal([]byte(message), &msg); err != nil {
		log.Printf("❌ Ошибка при разборе JSON: %v", err)
		return
	}

	switch msg.Type {
		case FindFreeSuperpeerMessageType: {
			log.Println("🔎 Обработка FIND_FREE_SUPERPEER")

			if len(a.peers) < a.maxPeers {
				log.Printf("✅ Я суперпир с доступным местом (%d/%d пиров)", len(a.peers), a.maxPeers)
				log.Printf("📤 Отправка FREE_SUPERPEER %s:%d", a.host, 55001)

				address := net.UDPAddr{IP: remoteAddr.IP, Port: 55001}

				body, err := json.Marshal(FreeSuperpeer{IP: a.host})
				if err != nil {
					log.Printf("Ошибка маршалинга структуры: %v", err)
					return
				}

				sendJSONUDP(address.String(), Message{Type: FreeSuperpeerMessageType, Body: body})
			} else {
				log.Printf("❌ Нет места у суперпира, пересылаю FIND_FREE_SUPERPEER другим суперпирам")
				superpeers := a.getSuperpeers()

				for superpeerIP := range superpeers {
					if superpeerIP == remoteAddr.String() {
						continue
					}
					
					address := net.UDPAddr{IP: net.ParseIP(superpeerIP), Port: 55001}
					log.Printf("➡️ Пересылка FIND_FREE_SUPERPEER на %s", address.String())
					sendJSONUDP(address.String(), Message{Type: FindFreeSuperpeerMessageType})
				}
			}
		}

		case FreeSuperpeerForBootstrapMessageType: {
			log.Println("🪜 Обработка FREE_SUPERPEER_FOR_BOOTSTRAP")

			var body FreeSuperpeerForBootstrap
			if err := json.Unmarshal(msg.Body, &body); err != nil {
				log.Printf("⚠️ Ошибка разбора тела: %v %s", err, msg.Body)
				return
			}
			log.Printf("📥 Получен адрес для bootstrap: %s", body.IP)

			log.Printf("🚀 Запуск bootstrap с %s", body.IP)

			a.bootstrap(BootstrapAddress{
				IP: body.IP,
			})
		}

		case FreeSuperpeerMessageType: {
			log.Println("📡 Обработка FREE_SUPERPEER")

			addr := strings.TrimSpace(strings.TrimPrefix(message, "FREE_SUPERPEER"))
			log.Printf("🔗 Адрес суперпира: %s", addr)

			for _, waitingPeerAddress := range a.waitingPeers {
				log.Printf("📨 Рассылка FREE_SUPERPEER_FOR_BOOTSTRAP %s -> %s", addr, waitingPeerAddress)

				address := net.UDPAddr{IP: remoteAddr.IP, Port: 55001}

				body, err := json.Marshal(FreeSuperpeerForBootstrap{IP: a.host})
				if err != nil {
					log.Printf("Ошибка маршалинга структуры: %v", err)
					return
				}

				sendJSONUDP(address.String(), Message{Type: FreeSuperpeerMessageType, Body: body})
			}

			var body FreeSuperpeer
			if err := json.Unmarshal(msg.Body, &body); err != nil {
				log.Printf("⚠️ Ошибка разбора тела: %v", err)
				return
			}

			for superpeerAddress := range a.getSuperpeers() {
				log.Printf("🔁 Пересылка FREE_SUPERPEER -> %s", superpeerAddress)

				nextBody, err := json.Marshal(body)
				if err != nil {
					log.Printf("Ошибка маршалинга структуры: %v", err)
					return
				}

				sendJSONUDP(superpeerAddress, Message{Type: FreeSuperpeerMessageType, Body: nextBody})
			}
		}

		case ElectNewSuperpeerMessageType: {
			peerCount := len(a.getPeers())
			log.Println(a.peers, peerCount)

			for peerIP := range a.getPeers() {
				address := net.UDPAddr{IP: net.ParseIP(peerIP), Port: 55001}

				sendJSONUDP(address.String(), Message{Type: CandidateSuperpeerMessageType})
			}
		}
	}
}

func (a *Agent) bootstrap(bootstrap BootstrapAddress) {
	log.Printf("Узел начал bootstraping-процесс")

	maxRetries := 5
	delay := 5 * time.Second

	for i := 1; i <= maxRetries; i++ {
		addr := fmt.Sprintf("%s:%d", bootstrap.IP, 55000)
		conn, err := net.Dial("tcp", addr)

		if err != nil {
			log.Printf("⚠ Попытка %d/%d — ошибка регистрации: %v", i, maxRetries, err)
			time.Sleep(delay)
			continue
		}

		sendJSONTCP(conn, Message{Type: ConnectRequestMessageType})
		log.Printf("Отправлен запрос на bootstrap на адрес %s", addr)

		response, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			log.Printf("Ошибка при чтении ответа суперпира: %v", err)
			conn.Close()
			time.Sleep(delay)
			continue
		}

		log.Printf("Ответ суперпира: %s", response)
		conn.Close()

		var message Message
		if err := json.Unmarshal([]byte(response), &message); err != nil {
			log.Printf("⚠️ Ошибка разбора сообщения: %v", err)
			return
		}

		if message.Type == ConnectedMessageType {
			a.peers[bootstrap.IP] = PeerInfo{
				IP: bootstrap.IP,
				IsSuper: true,
			}

			a.discoverPeers(bootstrap.IP)
			return
		} else if message.Type == WaitMessageType {
			log.Printf("⏳ Ожидание выбора нового суперпира...")
			return
		}
	}

	log.Printf("❌ Не удалось зарегистрироваться после %d попыток", maxRetries)
}

func (a *Agent) reportPeers(exceptionIPs []string) {
	peers := make(ReportPeersRequest, 0)

	for _, peerInfo := range a.peers {
		peers = append(peers, peerInfo)
	}

	body, err := json.Marshal(peers);
	if err != nil {
		log.Printf("⚠️ Ошибка разбора тела: %v", err)
		return
	}

	for peerIP := range a.peers {
		if (exceptionIPs == nil || !slices.Contains(exceptionIPs, peerIP)) {
			address := fmt.Sprintf("%s:%d", peerIP, 55001)
			sendJSONUDP(address, Message{Type: ReportPeersMessageType, Body: body })
		}
	}
}

func (a *Agent) discoverPeers(ip string) {
	address := fmt.Sprintf("%s:%d", ip, 55000)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("⚠ Ошибка при подключении к %s: %v", address, err)
		return
	}
	defer conn.Close()

	sendJSONTCP(conn, Message{Type: DiscoverPeersMessageType})

	response, _ := bufio.NewReader(conn).ReadString('\n')

	var peers DiscoverPeersResponse
	if err := json.Unmarshal([]byte(response), &peers); err != nil {
		log.Printf("⚠️ Ошибка разбора тела здесь: %v %s", err, response)
		return
	}

	a.peersMutex.Lock()

	for _, peerInfo := range peers {
		if peerInfo.IP != a.host {
			a.peers[peerInfo.IP] = peerInfo
		}
	}
	a.peersMutex.Unlock()
	log.Printf("✅ Получены узлы от %s: %v", ip, peers)
}
