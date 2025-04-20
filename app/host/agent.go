package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
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
			return
		} else if message.Type == WaitMessageType {
			log.Printf("⏳ Ожидание выбора нового суперпира...")
			return
		}
	}

	log.Printf("❌ Не удалось зарегистрироваться после %d попыток", maxRetries)
}