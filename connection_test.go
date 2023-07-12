package goreaction

import (
	"fmt"
	"io"
	"log"
	"net"
	"testing"
	"time"
)

type example struct {
}

func (s *example) OnConnect(c *Connection) {
	log.Println(" OnConnect ： ", c.PeerAddr())
	if err := c.Close(); err != nil {
		panic(err)
	}
}

func (s *example) OnMessage(c *Connection, ctx interface{}, data []byte) (out interface{}) {
	log.Println("OnMessage")

	return
}

func (s *example) OnClose(c *Connection) {
	log.Println("OnClose")
}

func TestConnClose(t *testing.T) {
	//log.SetLevel(log.LevelDebug)
	handler := new(example)

	s, err := NewServer(handler)
	//,
	//	Network("tcp"),
	//	Address("localhost:12345"),
	//	NumLoops(4),
	//	ReusePort(true))
	if err != nil {
		t.Fatal(err)
	}

	go s.Start()

	conn, err := net.DialTimeout("tcp", "127.0.0.1:12345", time.Second*5)
	if err != nil {
		log.Fatal(err)
		return
	}

	buf := make([]byte, 8)
	n, err := conn.Read(buf)
	if n != 0 || err != io.EOF {
		t.Fatal()
	}
	fmt.Println(buf)
	s.Stop()
}
