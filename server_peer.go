package main

import (
	"github.com/gorilla/websocket"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/wire"
)

// ServerPeer 代表一个服务器节点，每个服务器节点都会建立与其它服务器节点的接连，
// 这个对象用于处理跨服务节点消息收发。
type ServerPeer struct {
	conn   *websocket.Conn
	server *database.Server
	send   chan wire.Message
}

// NewServerPeer new ServerPeer
func NewServerPeer(conn *websocket.Conn,
	server *database.Server) *ServerPeer {
	return &ServerPeer{
		conn:   conn,
		server: server,
	}
}
