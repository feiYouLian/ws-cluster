package main

import (
	"github.com/gorilla/websocket"
	"github.com/ws-cluster/database"
)

// ServerPeer 代表一个服务器节点，每个服务器节点都会建立与其它服务器节点的接连，
// 这个对象用于处理跨服务节点消息收发。
type ServerPeer struct {
	// ID 服务器Id, 自动生成，缓存到file 中，重启时 ID 不变
	ID     string
	conn   *websocket.Conn
	server *database.Server
}

func NewServerPeer(config *Config) (*ServerPeer, error) {

	return &ServerPeer{}, nil
}
