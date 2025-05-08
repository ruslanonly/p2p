package threats

import (
	"encoding/json"
	"fmt"
	"log"
	"pkg/threats/model"

	ipc "github.com/james-barrow/golang-ipc"
)

// Threats IPC сервер осуществляет отправку информации о зловредных и подозрительных узлах.
// Также предлагает возможность заблокировать или помиловать (разблокировать) хост по его IP
type ThreatsIPCServer struct {
	server *ipc.Server
}

func NewThreatsIPCServer(pipeName string) (*ThreatsIPCServer, error) {
	s, err := ipc.StartServer(pipeName, nil)
	if err != nil {
		return nil, err
	}

	fmt.Println("📮 Запущен сервер")

	tm := &ThreatsIPCServer{
		server: s,
	}

	return tm, nil
}

func (tm *ThreatsIPCServer) message(msgType model.MessageType, body interface{}) {
	if marshalledBody, err := json.Marshal(body); err != nil {
		log.Printf("Ошибка при маршалинге сообщения для сообщения '%d': %v", msgType, err)
	} else {
		tm.server.Write(int(msgType), marshalledBody)
	}
}

func (tm *ThreatsIPCServer) RedTrafficMessage(body model.RedTrafficMessageTypeBody) {
	tm.message(model.RedTrafficMessageType, body)
}

func (tm *ThreatsIPCServer) YellowTrafficMessage(body model.YellowTrafficMessageTypeBody) {
	tm.message(model.YellowTrafficMessageType, body)
}

func (tm *ThreatsIPCServer) Listen(
	blockHostHandler func(body model.BlockHostMessageTypeBody),
	mercyHostHandler func(body model.MercyHostMessageTypeBody),
) {
	for {
		message, err := tm.server.Read()

		if err != nil {
			fmt.Printf("📮📮📮 Ошибка при чтении сообщения по IPC: %v\n", err)
		}

		fmt.Printf("📮📮📮 Новое сообщение по IPC: %v\n", message)

		if message.MsgType == int(model.BlockHostMessageType) {
			var body model.BlockHostMessageTypeBody
			if err := json.Unmarshal([]byte(message.Data), &body); err != nil {
				log.Println("Ошибка при парсинге сообщения:", err)
				continue
			}

			blockHostHandler(body)
		} else if message.MsgType == int(model.MercyHostMessageType) {
			var body model.MercyHostMessageTypeBody
			if err := json.Unmarshal([]byte(message.Data), &body); err != nil {
				log.Println("Ошибка при парсинге сообщения:", err)
				continue
			}

			mercyHostHandler(body)
		}
	}
}
