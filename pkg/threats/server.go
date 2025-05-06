package threats

import (
	"encoding/json"
	"fmt"
	"log"
	"pkg/threats/model"

	ipc "github.com/james-barrow/golang-ipc"
)

type ThreatsIPCServer struct {
	server *ipc.Client
}

func NewThreatsIPCServer() (*ThreatsIPCServer, error) {
	c, err := ipc.StartClient(PipeName, nil)
	if err != nil {
		return nil, err
	}

	tm := &ThreatsIPCServer{
		server: c,
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
) {
	for {
		message, err := tm.server.Read()

		if err != nil {
			fmt.Printf("📮 Ошибка при чтении сообщения по IPC: %v\n", err)
		}

		fmt.Printf("📮 Новое сообщение по IPC: %v\n", message)

		if message.MsgType == int(model.RedTrafficMessageType) {
			var body model.BlockHostMessageTypeBody
			if err := json.Unmarshal([]byte(message.Data), &body); err != nil {
				log.Println("Ошибка при парсинге сообщения:", err)
				return
			}

			blockHostHandler(body)
		}
	}
}
