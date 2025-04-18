package agent

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type PeerInfo struct {
	IP string
	IsSuper   bool
}

type Agent struct {
	host          string
	maxPeers      int
	peers         map[string]PeerInfo
	peersMutex    sync.Mutex
	waitingPeers  []string
}

type AgentConfiguration struct {
	Host string
	MaxPeers int
}

type Address struct {
	IP string
	Port int
}

func GetAddress(addrStr string) (Address, error) {
	parts := strings.Split(addrStr, ":")
	if len(parts) != 2 {
		return Address{}, fmt.Errorf("неверный формат адреса: %s", addrStr)
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return Address{}, fmt.Errorf("не удалось распарсить порт: %s", parts[1])
	}

	return Address{
		IP:   parts[0],
		Port: port,
	}, nil
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

func (a *Agent) Start(bootstrapAddress *Address) {
	go a.listenUDP()
	a.listenTCP(bootstrapAddress)
}


func (a *Agent) listenTCP(bootstrapAddress *Address) {
	addr := fmt.Sprintf("%s:%d", a.host, 55000)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Ошибка при запуске сервера: %v", err)
	}
	defer listener.Close()

	if (bootstrapAddress != nil) {
		log.Printf("🔥 Узел запущен по адресу %s", a.host)
		a.bootstrap(*bootstrapAddress)
	} else {
		log.Printf("🔥 Узел запущен по адресу %s и является суперпиром", a.host)
	}

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
	message, err := reader.ReadString('\n')

	if err != nil {
		log.Printf("Ошибка чтения: %v", err)
		return
	}

	message = strings.TrimSpace(message)
	remoteIP, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		log.Printf("Ошибка обработки адреса удаленного хоста: %v", err)
		return
	}

	log.Printf("📩 Получено TCP сообщение от %s: %s", remoteIP, message)

	switch {
	case strings.HasPrefix(message, "CONNECT_REQUEST"):
		a.peersMutex.Lock()
		if len(a.peers) < a.maxPeers {
			a.peers[remoteIP] = PeerInfo{
				IP: remoteIP,
				IsSuper: false,
			}
			a.peersMutex.Unlock()
			fmt.Fprintf(conn, "CONNECTED %s\n", a.host)
			log.Printf("✅ Подключён новый узел: %s", remoteIP)
		} else {
			a.peersMutex.Unlock()

			a.waitingPeers = append(a.waitingPeers, remoteIP)
			fmt.Fprintf(conn, "WAIT\n")
			log.Printf("⏳ Добавлен в очередь ожидания: %s", remoteIP)

			superpeers := a.getSuperpeers()

			if (len(superpeers) > 0) {
				log.Printf("⏳ Выполняется поиск суперпира среди собственных")

				for super := range superpeers {
					conn, err := net.Dial("udp", super)
					if err != nil {
						continue
					}

					defer conn.Close()

					_, err = conn.Write([]byte("FIND_SUPERPEER\n"))

					if err != nil {
						log.Printf("Ошибка отправки UDP: %v", err)
					}
				}
			} else {
				log.Printf("⏳ Инициируются выборы нового суперпира среди своих пиров")

				peers := a.getPeers()

				for peerIP := range peers {
					remoteAddr := &net.UDPAddr{
						IP: net.ParseIP(peerIP),
						Port: 55001,
					}

					conn, err := net.Dial("udp", remoteAddr.String())

					if err != nil {
						log.Println(remoteAddr.IP, remoteAddr.Port, remoteAddr.String())
						continue
					}
					
					defer conn.Close()

					log.Printf("Отправка ELECT_NEW_SUPERPEER узлу %s", peerIP)

					_, err = conn.Write([]byte("ELECT_NEW_SUPERPEER %s\n"))

					if err != nil {
						log.Printf("Ошибка отправки UDP: %v", err)
					}
				}
			}
	}
	case message == "DISCOVER_PEERS":
		a.peersMutex.Lock()
		peersList := make([]string, 0, len(a.peers))
		for peer := range a.peers {
			peersList = append(peersList, peer)
		}
		a.peersMutex.Unlock()
		response := strings.Join(peersList, "\n") + "\n"
		conn.Write([]byte(response))
		log.Printf("📨 Отправлен список узлов: %s", response)
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
	switch {
		case message == "FIND_FREE_SUPERPEER": {
			log.Println("🔎 Обработка FIND_FREE_SUPERPEER")

			if len(a.peers) < a.maxPeers {
				log.Printf("✅ Я суперпир с доступным местом (%d/%d пиров)", len(a.peers), a.maxPeers)
				log.Printf("📤 Отправка FREE_SUPERPEER %s:%d", a.host, 55001)

				address := net.UDPAddr{IP: remoteAddr.IP, Port: 55001}
				conn, err := net.Dial("udp", address.String())
				if err != nil {
					log.Printf("⚠️ Ошибка соединения с %s: %v", remoteAddr.String(), err)
					return
				}
				defer conn.Close()

				msg := fmt.Sprintf("FREE_SUPERPEER %s:%d\n", a.host, 55001)
				_, err = conn.Write([]byte(msg))

				if err != nil {
					log.Fatalf("Ошибка отправки UDP: %v", err)
				}
			} else {
				log.Printf("❌ Нет места у суперпира, пересылаю FIND_FREE_SUPERPEER другим суперпирам")
				superpeers := a.getSuperpeers()

				for addr := range superpeers {
					if addr == remoteAddr.String() {
						continue
					}

					log.Printf("➡️ Пересылка FIND_FREE_SUPERPEER на %s", addr)
					conn, err := net.Dial("udp", addr)
					if err != nil {
						log.Printf("⚠️ Ошибка при пересылке FIND_FREE_SUPERPEER на %s: %v", addr, err)
						continue
					}
					defer conn.Close()

					_, err = conn.Write([]byte("FIND_FREE_SUPERPEER\n"))

					if err != nil {
						log.Printf("Ошибка отправки UDP: %v", err)
					}
				}
			}
		}

		case strings.HasPrefix(message, "FREE_SUPERPEER_FOR_BOOTSTRAP"): {

			log.Println("🪜 Обработка FREE_SUPERPEER_FOR_BOOTSTRAP")

			addr := strings.TrimSpace(strings.TrimPrefix(message, "FREE_SUPERPEER_FOR_BOOTSTRAP"))
			log.Printf("📥 Получен адрес для bootstrap: %s", addr)

			address, err := GetAddress(addr)
			if err != nil {
				log.Printf("⚠️ Не удалось разобрать адрес: %v", err)
				return
			}

			log.Printf("🚀 Запуск bootstrap с %s:%d", address.IP, address.Port)
			a.bootstrap(address)
		}

		case strings.HasPrefix(message, "FREE_SUPERPEER"): {

			log.Println("📡 Обработка FREE_SUPERPEER")

			addr := strings.TrimSpace(strings.TrimPrefix(message, "FREE_SUPERPEER"))
			log.Printf("🔗 Адрес суперпира: %s", addr)

			for _, waitingPeerAddress := range a.waitingPeers {
				log.Printf("📨 Рассылка FREE_SUPERPEER_FOR_BOOTSTRAP %s -> %s", addr, waitingPeerAddress)

				conn, err := net.Dial("udp", waitingPeerAddress)
				if err != nil {
					log.Printf("⚠️ Ошибка отправки на %s: %v", waitingPeerAddress, err)
					continue
				}
				defer conn.Close()

				msg := fmt.Sprintf("FREE_SUPERPEER_FOR_BOOTSTRAP %s\n", addr)
				_, err = conn.Write([]byte(msg))

				if err != nil {
					log.Printf("Ошибка отправки UDP: %v", err)
				}
			}

			for superpeerAddress := range a.getSuperpeers() {
				log.Printf("🔁 Пересылка FREE_SUPERPEER -> %s", superpeerAddress)

				conn, err := net.Dial("udp", superpeerAddress)
				if err != nil {
					log.Printf("⚠️ Ошибка отправки на %s: %v", superpeerAddress, err)
					continue
				}
				defer conn.Close()

				_, err = conn.Write([]byte(message))

				if err != nil {
					log.Printf("Ошибка отправки UDP: %v", err)
				}
			}
		}

		case strings.HasPrefix(message, "ELECT_NEW_SUPERPEER"): {
			
		}
	}
}


func (a *Agent) bootstrap(bootstrap Address) {
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

		fmt.Fprintf(conn, "CONNECT_REQUEST\n")
		log.Printf("Отправлен запрос на bootstrap на адрес %s", addr)

		response, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			log.Printf("Ошибка при чтении ответа суперпира: %v", err)
			conn.Close()
			time.Sleep(delay)
			continue
		}

		response = strings.TrimSpace(response)
		log.Printf("Ответ суперпира: %s", response)
		conn.Close()

		if strings.HasPrefix(response, "CONNECTED") {
			a.peers[bootstrap.IP] = PeerInfo{
				IP: bootstrap.IP,
				IsSuper: true,
			}

			return
		} else if strings.HasPrefix(response, "WAIT") {
			log.Printf("⏳ Ожидание выбора нового суперпира...")
			return
		}
	}

	log.Printf("❌ Не удалось зарегистрироваться после %d попыток", maxRetries)
}


func (a *Agent) discoverPeers(peerAddr string) {
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		log.Printf("⚠ Ошибка при подключении к %s: %v", peerAddr, err)
		return
	}
	defer conn.Close()

	fmt.Fprintf(conn, "DISCOVER_PEERS\n")
	response, _ := bufio.NewReader(conn).ReadString('\n')
	peers := strings.Split(strings.TrimSpace(response), "\n")
	a.peersMutex.Lock()
	for _, peerAddress := range peers {
		if peerAddress != "" {
			a.peers[peerAddress] = PeerInfo{
				IP: peerAddress,
				IsSuper: false,
			}
		}
	}
	a.peersMutex.Unlock()
	log.Printf("✅ Получены узлы от %s: %v", peerAddr, peers)
}
