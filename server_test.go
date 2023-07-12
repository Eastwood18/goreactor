package goreaction

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type serverTest struct {
	Count atomic.Int64
}

func (s *serverTest) OnConnect(c *Connection) {
	s.Count.Add(1)
	// log.Println(" OnConnect ： ", c.PeerAddr())
}

func (s *serverTest) OnMessage(c *Connection, ctx interface{}, data []byte) (out interface{}) {
	// log.Println("OnMessage")

	// out = data
	msg := append([]byte{}, data...)
	if err := c.Send(msg); err != nil {
		panic(err)
	}
	return
}

func (s *serverTest) OnClose(c *Connection) {
	s.Count.Add(-1)
	// log.Println("OnClose")
}

func TestServer_Start(t *testing.T) {
	handler := new(serverTest)

	s, err := NewServer(handler)
	//,
	//	Network("tcp"),
	//	Address("localhost:12345"),
	//	NumLoops(4),
	//	ReusePort(true))
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(time.Second)
		wg := new(sync.WaitGroup)
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				startClient("tcp", ":1388")
				wg.Done()
			}()
		}

		wg.Wait()
		s.Stop()
	}()

	s.Start()
}

func startClient(network, addr string) {
	rand.Seed(time.Now().UnixNano())
	c, err := net.Dial(network, addr)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	rd := bufio.NewReader(c)
	duration := time.Duration((rand.Float64()*2+1)*float64(time.Second)) / 8
	start := time.Now()
	for time.Since(start) < duration {
		sz := rand.Int()%(1024*1024) + 1
		data := make([]byte, sz)
		if _, err := rand.Read(data); err != nil {
			panic(err)
		}
		if _, err := c.Write(data); err != nil {
			panic(err)
		}
		data2 := make([]byte, len(data))
		if _, err := io.ReadFull(rd, data2); err != nil {
			panic(err)
		}
		if string(data) != string(data2) {
			panic("mismatch")
		}
	}
}

func ExampleServer_RunAfter() {
	handler := new(serverTest)

	s, _ := NewServer(handler)
	//,
	//	Network("tcp"),
	//	Address("localhost:12345"),
	//	NumLoops(4),
	//	ReusePort(true))

	go s.Start()
	defer s.Stop()

	s.RunAfter(time.Second, func() {
		fmt.Println("RunAfter")
	})

	time.Sleep(2500 * time.Millisecond)

	// Output:
	// RunAfter
}

func ExampleServer_RunEvery() {
	handler := new(serverTest)

	s, err := NewServer(handler)
	//,
	//	Network("tcp"),
	//	Address("localhost:12345"),
	//	NumLoops(4),
	//	ReusePort(true))
	if err != nil {
		panic(err)
	}

	go s.Start()
	defer s.Stop()

	t := s.RunEvery(time.Second, func() {
		fmt.Println("EveryFunc")
	})

	time.Sleep(4500 * time.Millisecond)
	t.Stop()
	time.Sleep(4500 * time.Millisecond)

	// Output:
	// EveryFunc
	// EveryFunc
	// EveryFunc
	// EveryFunc
}

func TestServer_Stop(t *testing.T) {
	handler := new(serverTest)

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
	time.Sleep(time.Second)
	var (
		succ, fail atomic.Int32
	)
	wg := new(sync.WaitGroup)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		conn, err := net.Dial("tcp", ":1388")
		if err != nil {
			fail.Add(1)
			log.Fatal(err)

		} else {
			succ.Add(1)

		}

		conn.Close()
		wg.Done()
	}

	wg.Wait()
	log.Printf("Success: %d Failed: %d\n", succ.Load(), fail.Load())

	time.Sleep(time.Second * 2)
	count := handler.Count.Load()
	if count != 0 {
		t.Fatal(count)
	}

	s.Stop()
}
