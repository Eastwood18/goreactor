//go:build linux

package poller

import (
	"errors"
	"golang.org/x/sys/unix"
	"log"
	"runtime"
	"sync/atomic"
)

const readEvent = unix.EPOLLIN | unix.EPOLLPRI
const writeEvent = unix.EPOLLOUT

type Event uint32

const (
	waitEventsBegin       = 1024
	EventRead       Event = 0x1
	EventWrite      Event = 0x2
	EventErr        Event = 0x80
	EventNone       Event = 0
)

type Poller struct {
	fd       int
	eventFd  int
	buf      []byte
	running  atomic.Bool
	waitDone chan struct{}
}

func New() (*Poller, error) {
	fd, err := unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}
	r0, _, errno := unix.Syscall(unix.SYS_EVENTFD2, 0, 0, 0)
	if errno != 0 {
		_ = unix.Close(fd)
		return nil, errno
	}

	eventFd := int(r0)
	err = unix.EpollCtl(fd, unix.EPOLL_CTL_ADD, eventFd, &unix.EpollEvent{
		Events: unix.EPOLLIN,
		Fd:     int32(eventFd),
	})
	if err != nil {
		_ = unix.Close(fd)
		_ = unix.Close(eventFd)
		return nil, err
	}
	return &Poller{
		fd:       fd,
		eventFd:  eventFd,
		buf:      make([]byte, 8),
		waitDone: make(chan struct{}),
	}, nil
}

var wakeBytes = []byte{1, 0, 0, 0, 0, 0, 0, 0}

func (ep *Poller) Wake() error {
	_, err := unix.Write(ep.eventFd, wakeBytes)
	return err
}

func (ep *Poller) wakeHandlerRead() {
	n, err := unix.Read(ep.eventFd, ep.buf)
	if err != nil || n != 8 {
		log.Fatal("wakeHandlerRead", err, n)
	}
}

func (ep *Poller) Close() error {
	if !(ep.running.Load()) {
		return errors.New("poller instance is not running")
	}
	ep.running.Store(false)
	if err := ep.Wake(); err != nil {
		return err
	}
	<-ep.waitDone
	unix.Close(ep.fd)
	unix.Close(ep.eventFd)
	return nil
}

func (ep *Poller) add(fd int, events uint32) error {
	return unix.EpollCtl(ep.fd, unix.EPOLL_CTL_ADD, fd, &unix.EpollEvent{
		Events: events,
		Fd:     int32(fd),
	})
}

func (ep *Poller) AddRead(fd int) error {
	return ep.add(fd, readEvent)
}

func (ep *Poller) AddWrite(fd int) error {
	return ep.add(fd, writeEvent)
}

func (ep *Poller) Remove(fd int) error {
	return unix.EpollCtl(ep.fd, unix.EPOLL_CTL_DEL, fd, nil)
}

func (ep *Poller) modify(fd int, events uint32) error {
	return unix.EpollCtl(ep.fd, unix.EPOLL_CTL_MOD, fd, &unix.EpollEvent{
		Events: events,
		Fd:     int32(fd),
	})
}

func (ep *Poller) EnableReadWrite(fd int) error {
	return ep.modify(fd, readEvent|writeEvent)
}

func (ep *Poller) EnableRead(fd int) error {
	return ep.modify(fd, readEvent)
}

func (ep *Poller) EnableWrite(fd int) error {
	return ep.modify(fd, writeEvent)
}

func (ep *Poller) Poll(handle func(fd int, event Event)) {
	defer func() {
		close(ep.waitDone)
	}()

	events := make([]unix.EpollEvent, waitEventsBegin)
	var (
		wake bool
		msec int // events wait time, -1 means event trigger 0 means polling
	)
	ep.running.Store(true)
	for {
		n, err := unix.EpollWait(ep.fd, events, msec)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			log.Fatal("EpollWait", err)
		}
		// it means that when event trigger available, use msec=0 call, otherwise, use msec=-1 to minus polling waste
		// active switch msec
		if n <= 0 {
			msec = -1
			runtime.Gosched()
			continue
		}
		msec = 0

		for i := 0; i < n; i++ {
			fd := int(events[i].Fd)
			if fd != ep.eventFd {
				var rEvents Event
				if ((events[i].Events & unix.POLLHUP) != 0) && ((events[i].Events & unix.POLLIN) == 0) {
					rEvents |= EventErr
				}
				if (events[i].Events&unix.EPOLLERR != 0) || (events[i].Events&unix.EPOLLOUT != 0) {
					rEvents |= EventWrite
				}
				if events[i].Events&(unix.EPOLLIN|unix.EPOLLPRI|unix.EPOLLRDHUP) != 0 {
					rEvents |= EventRead
				}

				handle(fd, rEvents)
			} else {
				ep.wakeHandlerRead()
				wake = true
			}
		}

		if wake {
			handle(-1, 0)
			wake = false
			if !ep.running.Load() {
				return
			}
		}

		if n == len(events) {
			events = make([]unix.EpollEvent, 2*n)
		}
	}
}
