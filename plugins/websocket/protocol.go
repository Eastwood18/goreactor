package websocket

import (
	"errors"
	"github.com/gobwas/pool/pbytes"
	"goreaction"
	"goreaction/plugins/websocket/ws"
	"goreaction/ringbuffer"
	"log"
)

const (
	upgradedKey     = "gev_ws_upgraded"
	headerbufferKey = "gev_header_buf"
)

// Protocol websocket
type Protocol struct {
	upgrade *ws.Upgrader
}

func (p *Protocol) UnPacket(c *goreaction.Connection, buf *ringbuffer.RingBuffer) (ctx interface{}, out []byte) {
	_, ok := c.Get(upgradedKey)
	if !ok {
		var err error
		out, _, err = p.upgrade.Upgrade(c, buf)
		if err != nil {
			log.Fatal("Websocket Upgrade :", err)
			return
		}
		c.Set(upgradedKey, true)
		c.Set(headerbufferKey, pbytes.Get(0, ws.MaxHeaderSize-2))
	} else {
		bts, _ := c.Get(headerbufferKey)
		header, err := ws.VirtualReadHeader(bts.([]byte), buf)
		if err != nil {
			if !errors.Is(err, ws.ErrHeaderNotReady) {
				log.Fatal(err)
			}
			return
		}
		if buf.VirtualLength() >= int(header.Length) {
			buf.VirtualFlush()

			payload := make([]byte, int(header.Length))
			buf.Read(payload)

			if header.Masked {
				ws.Cipher(payload, header.Mask, 0)
			}

			ctx = &header
			out = payload
		} else {
			buf.VirtualRevert()
		}
	}
	return
}

func (p *Protocol) Packet(c *goreaction.Connection, data interface{}) []byte {
	return data.([]byte)
}

func New(u *ws.Upgrader) *Protocol {
	return &Protocol{upgrade: u}
}
