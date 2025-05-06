package threats

import (
	"encoding/json"
	"fmt"
	"log"
	"pkg/threats/model"

	ipc "github.com/james-barrow/golang-ipc"
)

type ThreatsIPCClient struct {
	client *ipc.Client
}

func NewThreatsIPCClient() (*ThreatsIPCClient, error) {
	c, err := ipc.StartClient(PipeName, nil)
	if err != nil {
		return nil, err
	}

	tm := &ThreatsIPCClient{
		client: c,
	}

	return tm, nil
}

func (tm *ThreatsIPCClient) message(msgType model.MessageType, body interface{}) {
	if marshalledBody, err := json.Marshal(body); err != nil {
		log.Printf("Ошибка при маршалинге сообщения для сообщения '%d': %v", msgType, err)
	} else {
		tm.client.Write(int(msgType), marshalledBody)
	}
}

func (tm *ThreatsIPCClient) BlockHostMessage(body model.BlockHostMessageTypeBody) {
	tm.message(model.BlockHostMessageType, body)
}

func (tm *ThreatsIPCClient) Listen(
	redTrafficHandler func(body model.RedTrafficMessageTypeBody),
	yellowTrafficHandler func(body model.YellowTrafficMessageTypeBody),
) {
	for {
		message, err := tm.client.Read()

		if err != nil {
			fmt.Printf("📮 Ошибка при чтении сообщения по IPC: %v\n", err)
		}

		fmt.Printf("📮 Новое сообщение по IPC: %v\n", message)

		if message.MsgType == int(model.RedTrafficMessageType) {
			var body model.RedTrafficMessageTypeBody
			if err := json.Unmarshal([]byte(message.Data), &body); err != nil {
				log.Println("Ошибка при парсинге сообщения:", err)
				return
			}

			redTrafficHandler(body)
		} else if message.MsgType == int(model.YellowTrafficMessageType) {
			var body model.YellowTrafficMessageTypeBody
			if err := json.Unmarshal([]byte(message.Data), &body); err != nil {
				log.Println("Ошибка при парсинге сообщения:", err)
				return
			}

			yellowTrafficHandler(body)
		}

	}
}
