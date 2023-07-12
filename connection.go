package goreaction

import (
	"errors"
	"fmt"
	"github.com/RussellLuo/timingwheel"
	"golang.org/x/sys/unix"
	"goreaction/eventloop"
	"goreaction/poller"
	"goreaction/ringbuffer"
	"log"
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

type Callback interface {
	OnMessage(c *Connection, ctx interface{}, data []byte) interface{}
	OnClose(c *Connection)
}

type Connection struct {
	outBufLen  atomic.Int64
	inBufLen   atomic.Int64
	activeTime atomic.Int64
	fd         int
	connected  atomic.Bool
	buf        *ringbuffer.RingBuffer
	outBuf     *ringbuffer.RingBuffer
	inBuf      *ringbuffer.RingBuffer
	callback   Callback
	loop       *eventloop.EventLoop
	peerAddr   string
	ctx        interface{}
	//KeyValueContext

	idleTime    time.Duration
	timingWheel *timingwheel.TimingWheel
	timer       atomic.Value
	protocol    string
}

var ErrConnectionClosed = errors.New("connection closed")

func NewConnection(fd int,
	loop *eventloop.EventLoop,
	sa unix.Sockaddr,
	protocol string,
	tw *timingwheel.TimingWheel,
	idleTime time.Duration,
	back Callback) *Connection {
	conn := &Connection{
		fd:          fd,
		peerAddr:    sockAddrToString(sa),
		outBuf:      ringbuffer.GetFromPool(),
		inBuf:       ringbuffer.GetFromPool(),
		callback:    back,
		loop:        loop,
		idleTime:    idleTime,
		timingWheel: tw,
		protocol:    protocol,
		buf:         ringbuffer.New(0),
	}
	conn.connected.Store(true)

	if conn.idleTime > 0 {
		_ = conn.activeTime.Swap(time.Now().Unix())
		timer := conn.timingWheel.AfterFunc(conn.idleTime, conn.closeTimeoutConn())
		conn.timer.Store(timer)
	}
	return conn
}

func (c *Connection) UserBuffer() *[]byte {
	return c.loop.UserBuf
}

func (c *Connection) closeTimeoutConn() func() {
	return func() {
		now := time.Now()
		intervals := now.Sub(time.Unix(c.activeTime.Load(), 0))
		if intervals >= c.idleTime {
			_ = c.Close()
		} else {
			if c.connected.Load() {
				timer := c.timingWheel.AfterFunc(c.idleTime-intervals, c.closeTimeoutConn())
				c.timer.Store(timer)
			}
		}
	}
}

func (c *Connection) Context() interface{} {
	return c.ctx
}

func (c *Connection) SetContext(ctx interface{}) {
	c.ctx = ctx
}

func (c *Connection) PeerAddr() string {
	return c.peerAddr
}

func (c *Connection) Connected() bool {
	return c.connected.Load()
}

func (c *Connection) Send(data []byte) error {
	if !c.connected.Load() {
		return ErrConnectionClosed
	}
	c.loop.QueueInLoop(func() {
		if c.connected.Load() {
			c.sendInLoop(data)
		}
	})
	return nil
}

func (c *Connection) Close() error {
	if !c.connected.Load() {
		return ErrConnectionClosed
	}

	c.loop.QueueInLoop(func() {
		c.handleClose(c.fd)
	})
	return nil
}

func (c *Connection) ShutdownWrite() error {
	return unix.Shutdown(c.fd, unix.SHUT_WR)
}

func (c *Connection) ReadBufLen() int64 {
	return c.inBufLen.Load()
}

func (c *Connection) WriteBufLen() int64 {
	return c.outBufLen.Load()
}

// internal use, eventloop callback
func (c *Connection) HandleEvent(fd int, events poller.Event) {
	if c.idleTime > 0 {
		_ = c.activeTime.Swap(time.Now().Unix())
	}

	if events&poller.EventErr != 0 {
		c.handleClose(fd)
		return
	}

	if !c.outBuf.IsEmpty() {
		if events&poller.EventWrite != 0 {
			if c.handleWrite(fd) {
				return
			}
			if c.outBuf.IsEmpty() {
				c.outBuf.Reset()
			}
		}
	} else if events&poller.EventRead != 0 {
		if c.handleRead(fd) {
			return
		}
		if c.inBuf.IsEmpty() {
			c.inBuf.Reset()
		}
	}

	c.inBufLen.Swap(int64(c.inBuf.Length()))
	c.outBufLen.Swap(int64(c.outBuf.Length()))
}

func sockAddrToString(sa unix.Sockaddr) string {
	switch sa := (sa).(type) {
	case *unix.SockaddrInet4:
		return net.JoinHostPort(net.IP(sa.Addr[:]).String(), strconv.Itoa(sa.Port))
	case *unix.SockaddrInet6:
		return net.JoinHostPort(net.IP(sa.Addr[:]).String(), strconv.Itoa(sa.Port))
	default:
		return fmt.Sprintf("(unknown - %T)", sa)
	}
}

func (c *Connection) handleClose(fd int) {
	if c.connected.Load() {
		c.connected.Store(false)
		c.loop.DeleteFdInLoop(fd)
		c.callback.OnClose(c)
		if err := unix.Close(fd); err != nil {
			log.Fatal("[close fd]", err)
		}

		ringbuffer.PutInPool(c.inBuf)
		ringbuffer.PutInPool(c.outBuf)

		if v := c.timer.Load(); v != nil {
			timer := v.(*timingwheel.Timer)
			timer.Stop()
		}
	}
}

func (c *Connection) handleRead(fd int) (closed bool) {
	buf := c.loop.PacketBuf()
	n, err := unix.Read(c.fd, buf)
	if n == 0 || err != nil {
		if err != unix.EAGAIN {
			c.handleClose(fd)
			closed = true
		}
		return
	}

	if c.inBuf.IsEmpty() {
		c.buf.WithData(buf[:n])
		buf = buf[n:n]
		// c.handleProtocol

		recvData := c.recv()
		if len(recvData) != 0 {
			sendData := c.callback.OnMessage(c, nil, recvData)
			if sendData != nil {
				buf = append(buf, sendData.([]byte)...)
			}
		}

		if !c.buf.IsEmpty() {
			ft, _ := c.buf.PeekAll()
			c.inBuf.Write(ft)
		}
	} else {
		c.inBuf.Write(buf[:n])
		buf = buf[:0]
		// c.handleProtocol
		recvData := c.recv()
		if len(recvData) != 0 {
			sendData := c.callback.OnMessage(c, nil, recvData)
			if sendData != nil {
				buf = append(buf, sendData.([]byte)...)
			}
		}

	}
	if len(buf) != 0 {
		closed = c.sendInLoop(buf)
	}

	return
}

func (c *Connection) recv() []byte {
	s, e := c.buf.PeekAll()
	if len(e) > 0 {
		size := len(s) + len(e)
		userBuf := *c.UserBuffer()
		if size > cap(userBuf) {
			userBuf = make([]byte, size)
			*c.UserBuffer() = userBuf
		}
		copy(userBuf, s)
		copy(userBuf[len(s):], e)
		c.buf.RetrieveAll()

		return userBuf[:size]
	} else {
		c.buf.RetrieveAll()

		return s
	}
}

func (c *Connection) handleWrite(fd int) (closed bool) {
	ft, ed := c.outBuf.PeekAll()
	n, err := unix.Write(c.fd, ft)
	if err != nil {
		if err == unix.EAGAIN {
			return
		}
		c.handleClose(fd)
		closed = true
		return
	}
	c.outBuf.Retrieve(n)

	if n == len(ft) && len(ed) > 0 {
		n, err = unix.Write(c.fd, ed)
		if err != nil {
			if err == unix.EAGAIN {
				return
			}
			c.handleClose(fd)
			closed = true
			return
		}
		c.outBuf.Retrieve(n)
	}

	if c.outBuf.IsEmpty() {
		if err := c.loop.EnableRead(fd); err != nil {
			log.Fatal("[enableRead]", err)
		}
	}

	return
}

func (c *Connection) sendInLoop(data []byte) (closed bool) {
	if !c.outBuf.IsEmpty() {
		c.outBuf.Write(data)
	} else {
		n, err := unix.Write(c.fd, data)
		if err != nil && err != unix.EAGAIN {
			c.handleClose(c.fd)
			closed = true
			return
		}
		if n <= 0 {
			c.outBuf.Write(data)
		} else if n < len(data) {
			c.outBuf.Write(data[n:])
		}
		if !c.outBuf.IsEmpty() {
			c.loop.EnableReadWrite(c.fd)
		}
	}
	return
}
