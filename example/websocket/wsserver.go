package main

import (
	"goreaction"
	"goreaction/plugins/websocket"
	"goreaction/plugins/websocket/ws"
)

// NewWebSocketServer 创建 WebSocket Server
func NewWebSocketServer(handler websocket.WSHandler, u *ws.Upgrader, opts ...goreaction.Option) (server *goreaction.Server, err error) {
	opts = append(opts, goreaction.CustomProtocol(websocket.New(u)))
	return goreaction.NewServer(websocket.NewHandlerWrap(u, handler), opts...)
}
