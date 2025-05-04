package ipc

import (
	"fmt"

	"pkg/ipc"

	gipc "github.com/james-barrow/golang-ipc"
)

type TrafficModule struct {
	client *gipc.Client
}

func NewTrafficModule() (*TrafficModule, error) {
	c, err := gipc.StartClient(ipc.PipeName, nil)
	if err != nil {
		return nil, err
	}

	tm := &TrafficModule{
		client: c,
	}

	return tm, nil
}

type TrafficModuleHandler = func(msgType ipc.MessageType, data []byte)

func (tm *TrafficModule) Listen(handler TrafficModuleHandler) {
	for {
		message, err := tm.client.Read()

		if err != nil {
			fmt.Printf("📮 Ошибка при чтении сообщения по IPC: %v\n", err)
		}

		fmt.Printf("📮 Новое сообщение по IPC: %v\n", message)
		handler(ipc.MessageType(message.MsgType), message.Data)
	}
}
