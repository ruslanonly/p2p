package main

import (
	"net"
)

func main() {
	conn, err := net.Dial("udp", "localhost:5001")
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	conn.Write([]byte("ping\n"))
}