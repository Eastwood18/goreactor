package main

import (
	"fmt"
	"net"
	"time"
)

var (
	addr = "127.0.0.1:9091"
)

func main() {
	conn, err := net.Dial("tcp", addr)
	defer conn.Close()
	if err != nil {
		fmt.Println("dial err", err)
		return
	}

	for {
		time.Sleep(time.Second)
		conn.Write([]byte("hello world"))
		fmt.Println("---")
	}
}
