package eventloop

import (
	"goreaction/poller"
	"goreaction/utils"
	"log"
	"sync/atomic"
	"unsafe"
)

var (
	DefaultPacketSize    = 65536
	DefaultBufferSize    = 4096
	DefaultTaskQueueSize = 1024
)

type Socket interface {
	HandleEvent(fd int, events poller.Event)
	Close() error
}

type EventLoop struct {
	eventLoopLocal
	// nolint
	// Prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	pad [128 - unsafe.Sizeof(eventLoopLocal{})%128]byte
}

type eventLoopLocal struct {
	ConnCnt    atomic.Int64
	needWake   *atomic.Bool
	poll       *poller.Poller
	mu         utils.SpinLock
	sockets    map[int]Socket
	packet     []byte
	taskQueueW []func()
	taskQueueR []func()

	UserBuf *[]byte
}

func New() (*EventLoop, error) {
	p, err := poller.New()
	if err != nil {
		return nil, err
	}
	userBuf := make([]byte, DefaultBufferSize)
	nw := new(atomic.Bool)
	nw.Store(true)
	return &EventLoop{
		eventLoopLocal: eventLoopLocal{
			poll:       p,
			packet:     make([]byte, DefaultPacketSize),
			sockets:    make(map[int]Socket),
			UserBuf:    &userBuf,
			needWake:   nw,
			taskQueueW: make([]func(), 0, DefaultTaskQueueSize),
			taskQueueR: make([]func(), 0, DefaultTaskQueueSize),
		},
	}, nil
}

func (el *EventLoop) PacketBuf() []byte {
	return el.packet
}

func (el *EventLoop) ConnectionCount() int64 {
	return el.ConnCnt.Load()
}

func (el *EventLoop) DeleteFdInLoop(fd int) {
	if err := el.poll.Remove(fd); err != nil {
		log.Fatal("[DeleteFdInLoop]", err)
	}
	delete(el.sockets, fd)
	el.ConnCnt.Add(-1)
}

func (el *EventLoop) AddSocketAndEnableRead(fd int, s Socket) error {
	el.sockets[fd] = s
	if err := el.poll.AddRead(fd); err != nil {
		delete(el.sockets, fd)
		return err
	}

	el.ConnCnt.Add(1)
	return nil
}

func (el *EventLoop) EnableReadWrite(fd int) error {
	return el.poll.EnableReadWrite(fd)
}

func (el *EventLoop) EnableRead(fd int) error {
	return el.poll.EnableRead(fd)
}

func (el *EventLoop) Run() {
	el.poll.Poll(el.handlerEvent)
}

func (el *EventLoop) Stop() error {
	el.QueueInLoop(func() {
		for _, v := range el.sockets {
			if err := v.Close(); err != nil {
				log.Fatal(err)
			}
		}
		el.sockets = nil
	})

	el.ConnCnt.Swap(0)
	return el.poll.Close()
}

func (el *EventLoop) QueueInLoop(f func()) {
	el.mu.Lock()
	el.taskQueueW = append(el.taskQueueW, f)
	el.mu.Unlock()

	if el.needWake.CompareAndSwap(true, false) {
		if err := el.poll.Wake(); err != nil {
			log.Fatal("QueueInLoop Wake loop, ", err)
		}
	}
}
func (el *EventLoop) handlerEvent(fd int, events poller.Event) {
	if fd != -1 {
		s, ok := el.sockets[fd]
		if ok {
			s.HandleEvent(fd, events)
		}
	} else {
		el.needWake.Store(true)
		el.doPendingFunc()
	}
}
func (el *EventLoop) doPendingFunc() {
	el.mu.Lock()
	el.taskQueueW, el.taskQueueR = el.taskQueueR, el.taskQueueW
	el.mu.Unlock()

	length := len(el.taskQueueR)
	for i := 0; i < length; i++ {
		el.taskQueueR[i]()
	}

	el.taskQueueR = el.taskQueueR[:0]
}
