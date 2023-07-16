package main

import (
	"flag"
	"goreaction"
	"goreaction/plugins/websocket/ws"
	"goreaction/plugins/websocket/ws/utils"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var (
	keyRequestHeader = "requestHeader"
	keyUri           = "uri"
)

type example struct {
	sync.Mutex
	sessions map[*goreaction.Connection]*Session
}

type Session struct {
	first  bool
	header http.Header
	conn   *goreaction.Connection
}

// connection lifecycle
// OnConnect() -> OnRequest() -> OnHeader() -> OnMessage() -> OnClose()

func (s *example) OnConnect(c *goreaction.Connection) {
	log.Println("OnConnect: ", c.PeerAddr())

	s.Lock()
	defer s.Unlock()

	s.sessions[c] = &Session{
		first: true,
		conn:  c,
	}
}

func (s *example) OnMessage(c *goreaction.Connection, data []byte) (messageType ws.MessageType, out []byte) {
	log.Println("OnMessage: ", string(data))

	s.Lock()
	session, ok := s.sessions[c]
	if !ok {
		s.Unlock()
		return
	}
	s.Unlock()

	if session.first {
		session.first = false

		_header, ok := c.Get(keyRequestHeader)
		if ok {
			header := _header.(http.Header)
			session.header = header
			log.Printf("request header header: %+v \n", header)
		}

		_uri, ok := c.Get(keyUri)
		if ok {
			uri := _uri.(string)
			log.Printf("request uri: %v \n", uri)
		}
	}

	messageType = ws.MessageBinary
	switch rand.Int() % 4 {
	case 0:
		out = data
	case 1:
		msg, err := utils.PackData(ws.MessageText, data)
		if err != nil {
			panic(err)
		}
		if err := c.Send(msg); err != nil {
			msg, err := utils.PackCloseData(err.Error())
			if err != nil {
				panic(err)
			}
			if e := c.Send(msg); e != nil {
				panic(e)
			}
		}
	case 2:
		msg, err := utils.PackCloseData("close")
		if err != nil {
			panic(err)
		}
		if e := c.Send(msg); e != nil {
			panic(e)
		}
	case 3:
		// async send message
		var count = 10
		for i := 0; i < count; i++ {
			go func() {
				msg, err := utils.PackData(ws.MessageText, []byte("async write data"))
				if err != nil {
					panic(err)
				}
				if e := c.Send(msg); e != nil {
					panic(e)
				}
			}()
		}
	}
	return
}

func (s *example) OnClose(c *goreaction.Connection) {
	log.Println("OnClose")

	s.Lock()
	defer s.Unlock()

	delete(s.sessions, c)
}

func loopBoardcast(serv *example) {
	for {
		serv.Lock()

		for _, session := range serv.sessions {
			if session == nil {
				serv.Unlock()
				continue
			}

			msg, err := utils.PackData(ws.MessageText, []byte("publish message"))
			if err != nil {
				serv.Unlock()
				continue
			}
			_ = session.conn.Send(msg)
		}
		serv.Unlock()

		time.Sleep(200 * time.Millisecond)
	}
}

func main() {
	var (
		port  int
		loops int
	)

	flag.IntVar(&port, "port", 1833, "server port")
	flag.IntVar(&loops, "loops", -1, "num loops")
	flag.Parse()

	handler := &example{
		sessions: make(map[*goreaction.Connection]*Session, 10),
	}

	wsUpgrader := &ws.Upgrader{}
	wsUpgrader.OnHeader = func(c *goreaction.Connection, key, value []byte) error {
		log.Println("OnHeader: ", string(key), string(value))

		var header http.Header
		_header, ok := c.Get("requestHeader")
		if ok {
			header = _header.(http.Header)
		} else {
			header = make(http.Header)
		}
		header.Set(string(key), string(value))

		c.Set(keyRequestHeader, header)
		return nil
	}

	wsUpgrader.OnRequest = func(c *goreaction.Connection, uri []byte) error {
		log.Println("OnRequest: ", string(uri))

		c.Set(keyUri, string(uri))
		return nil
	}

	go loopBoardcast(handler)

	s, err := NewWebSocketServer(handler, wsUpgrader,
		goreaction.Network("tcp"),
		goreaction.Address(":"+strconv.Itoa(port)),
		goreaction.NumLoops(loops))
	if err != nil {
		panic(err)
	}

	s.Start()
}
