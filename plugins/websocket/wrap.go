package websocket

import (
	"github.com/gobwas/pool/pbytes"
	"goreaction"
	"goreaction/plugins/websocket/ws"
	"goreaction/plugins/websocket/ws/utils"
	"log"
)

type WSHandler interface {
	OnConnect(c *goreaction.Connection)
	OnMessage(c *goreaction.Connection, msg []byte) (ws.MessageType, []byte)
	OnClose(c *goreaction.Connection)
}

// HandlerWrap goreaction Handler wrap
type HandlerWrap struct {
	wsHandler WSHandler
	Upgrade   *ws.Upgrader
}

// NewHandlerWrap websocket handler wrap
func NewHandlerWrap(u *ws.Upgrader, wsHandler WSHandler) *HandlerWrap {
	return &HandlerWrap{
		wsHandler: wsHandler,
		Upgrade:   u,
	}
}

// OnConnect wrap
func (s *HandlerWrap) OnConnect(c *goreaction.Connection) {
	s.wsHandler.OnConnect(c)
}

// OnMessage wrap
func (s *HandlerWrap) OnMessage(c *goreaction.Connection, ctx interface{}, payload []byte) interface{} {
	header, ok := ctx.(*ws.Header)
	if !ok && len(payload) != 0 { // 升级协议 握手
		return payload
	}

	if ok {
		if header.OpCode.IsControl() {
			var (
				out []byte
				err error
			)
			switch header.OpCode {
			case ws.OpClose:
				out, err = utils.HandleClose(header, payload)
				if err != nil {
					log.Fatal(err)
				}
				_ = c.ShutdownWrite()
			case ws.OpPing:
				out, err = utils.HandlePing(payload)
				if err != nil {
					log.Fatal(err)
				}
			case ws.OpPong:
				out, err = utils.HandlePong(payload)
				if err != nil {
					log.Fatal(err)
				}
			}
			return out
		}

		messageType, out := s.wsHandler.OnMessage(c, payload)
		if len(out) > 0 {
			var frame *ws.Frame
			switch messageType {
			case ws.MessageBinary:
				frame = ws.NewBinaryFrame(out)
			case ws.MessageText:
				frame = ws.NewTextFrame(out)
			}
			var err error
			out, err = ws.FrameToBytes(frame)
			if err != nil {
				log.Fatal(err)
			}

			return out
		}
	}
	return nil
}

// OnClose wrap
func (s *HandlerWrap) OnClose(c *goreaction.Connection) {
	s.wsHandler.OnClose(c)

	if bts, ok := c.Get(headerbufferKey); ok {
		pbytes.Put(bts.([]byte))
	}
}
