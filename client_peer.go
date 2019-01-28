package main

import (
	"github.com/gorilla/websocket"
	"github.com/ws-cluster/database"
)

// ClientPeer 代表一个客户端节点，消息收发的处理逻辑
type ClientPeer struct {
	conn   *websocket.Conn
	client *database.Client
}

// NewClientPeer create ClientPeer
func NewClientPeer(conn *websocket.Conn, client *database.Client) (*ClientPeer, error) {
	return &ClientPeer{
		conn:   conn,
		client: client,
	}, nil
}
