package main

import (
	"goreaction"
	"log"
	"net/http"
	"time"
)

type example struct {
}

func (s *example) OnConnect(c *goreaction.Connection) {
	log.Println(" OnConnect ： ", c.PeerAddr())
}
func (s *example) OnMessage(c *goreaction.Connection, ctx interface{}, data []byte) (out interface{}) {
	log.Println("OnMessage from : %s", c.PeerAddr())
	out = data
	return
}

func (s *example) OnClose(c *goreaction.Connection) {
	log.Println("OnClose: ", c.PeerAddr())
}

func main() {
	go func() {
		if err := http.ListenAndServe(":6060", nil); err != nil {
			panic(err)
		}
	}()

	handler := new(example)

	s, err := goreaction.NewServer(handler,
		goreaction.Network("tcp"),
		goreaction.Address(":8899"),
		goreaction.NumLoops(4),
		goreaction.IdleTime(5*time.Second))
	if err != nil {
		panic(err)
	}

	s.Start()
}
