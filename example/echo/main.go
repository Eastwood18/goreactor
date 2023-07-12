package main

import (
	"goreaction"
	"log"
	"sync/atomic"
	"time"
)

type echoExample struct {
	Count atomic.Int64
}

func (s *echoExample) OnConnect(c *goreaction.Connection) {
	s.Count.Add(1)
	log.Println(" OnConnect ： ", c.PeerAddr())
}
func (s *echoExample) OnMessage(c *goreaction.Connection, ctx interface{}, data []byte) (out interface{}) {
	log.Println("OnMessage", string(data))
	out = data
	return
}

func (s *echoExample) OnClose(c *goreaction.Connection) {
	s.Count.Add(-1)
	log.Println("OnClose")
}

func main() {
	handler := new(echoExample)

	s, err := goreaction.NewServer(handler,
		goreaction.Network("tcp"),
		goreaction.Address(":8899"),
		goreaction.NumLoops(4),
		goreaction.ReusePort(true))
	if err != nil {
		log.Panicln(err)
	}

	s.RunEvery(time.Second*5, func() {
		log.Println("connections :", handler.Count.Load())
	})

	s.Start()
}
