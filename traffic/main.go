package main

import (
	"log"
	"time"

	"pkg/ipc"

	gipc "github.com/james-barrow/golang-ipc"
)

func main() {
	ipcServer, err := gipc.StartServer(ipc.PipeName, nil)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("IPC запущен")

	time.Sleep(10 * time.Second)
	ipcServer.Write(1, []byte("Hello"))

	select {}
}
