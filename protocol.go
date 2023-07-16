package goreaction

import "goreaction/ringbuffer"

type Protocol interface {
	UnPacket(c *Connection, buf *ringbuffer.RingBuffer) (interface{}, []byte)
	Packet(c *Connection, data interface{}) []byte
}

type DefaultProtocol struct{}

func (d *DefaultProtocol) UnPacket(c *Connection, buf *ringbuffer.RingBuffer) (interface{}, []byte) {
	s, e := buf.PeekAll()
	if len(e) > 0 {
		size := len(s) + len(e)
		userBuf := *c.UserBuffer()
		if size > cap(userBuf) {
			userBuf = make([]byte, size)
			*c.UserBuffer() = userBuf
		}

		copy(userBuf, s)
		copy(userBuf[len(s):], e)
		buf.RetrieveAll()

		return nil, userBuf[:size]
	} else {
		buf.RetrieveAll()

		return nil, s
	}
}

func (d *DefaultProtocol) Packet(c *Connection, data interface{}) []byte {
	return data.([]byte)
}
