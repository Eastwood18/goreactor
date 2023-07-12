package goreaction

import (
	"errors"
	"github.com/RussellLuo/timingwheel"
	"golang.org/x/sys/unix"
	"goreaction/eventloop"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type Handler interface {
	Callback
	OnConnect(c *Connection)
}

type Server struct {
	listener  *listener
	workLoops []*eventloop.EventLoop
	callback  Handler

	timingWheel *timingwheel.TimingWheel
	opts        *Options
	running     atomic.Bool
}

type scheduler struct {
	interval time.Duration
}

func (s *scheduler) Next(prev time.Time) time.Time {
	return prev.Add(s.interval)
}

func NewServer(handler Handler, opts ...Option) (server *Server, err error) {
	if handler == nil {
		return nil, errors.New("handler is nil")
	}
	options := newOptions(opts...)
	server = new(Server)
	server.callback = handler
	server.opts = options
	server.timingWheel = timingwheel.NewTimingWheel(server.opts.tick, server.opts.wheelSize)
	server.listener, err = newListener(server.opts.Network, server.opts.Address, options.ReusePort, server.handleNewConnection)

	if err != nil {
		return nil, err
	}
	if server.opts.NumLoops <= 0 {
		server.opts.NumLoops = runtime.NumCPU()
	}

	wloops := make([]*eventloop.EventLoop, server.opts.NumLoops)
	for i := 0; i < server.opts.NumLoops; i++ {
		l, err := eventloop.New()
		if err != nil {
			for j := 0; j < i; j++ {
				wloops[j].Stop()
			}
			return nil, err
		}
		wloops[i] = l
	}
	server.workLoops = wloops
	return
}

func (s *Server) RunAfter(d time.Duration, f func()) *timingwheel.Timer {
	return s.timingWheel.AfterFunc(d, f)
}

func (s *Server) RunEvery(d time.Duration, f func()) *timingwheel.Timer {
	return s.timingWheel.ScheduleFunc(&scheduler{d}, f)
}

func (s *Server) handleNewConnection(fd int, sa unix.Sockaddr) {
	loadBalance := RoundRobin()
	loop := loadBalance(s.workLoops)
	c := NewConnection(fd, loop, sa, "", s.timingWheel, s.opts.IdleTime, s.callback)

	loop.QueueInLoop(func() {
		s.callback.OnConnect(c)
		if err := loop.AddSocketAndEnableRead(fd, c); err != nil {
			log.Fatal("[AddSocketAndEnableRead]", err)
		}
	})
}

func RoundRobin() func([]*eventloop.EventLoop) *eventloop.EventLoop {
	var nextLoopIndex int

	return func(loops []*eventloop.EventLoop) *eventloop.EventLoop {
		l := loops[nextLoopIndex]
		nextLoopIndex = (nextLoopIndex + 1) % len(loops)
		return l
	}
}

func (s *Server) Start() {
	wg := new(sync.WaitGroup)
	s.timingWheel.Start()

	l := len(s.workLoops)
	for i := 0; i < l; i++ {
		wg.Add(1)
		go func(i int) {
			s.workLoops[i].Run()
			wg.Done()
		}(i)
	}

	wg.Add(1)
	go func() {
		s.listener.Run()
		wg.Done()
	}()

	s.running.Store(true)
	wg.Wait()
}

func (s *Server) Stop() {
	if s.running.Load() {
		s.running.Store(false)
		s.timingWheel.Stop()
		if err := s.listener.Stop(); err != nil {
			log.Fatal(err)
		}

		for i := range s.workLoops {
			if err := s.workLoops[i].Stop(); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func (s *Server) Options() Options {
	return *s.opts
}
