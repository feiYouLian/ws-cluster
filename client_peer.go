package main

import (
	"github.com/gorilla/websocket"
	"github.com/ws-cluster/db"
)

// ClientPeer 代表一个客户端节点，消息收发的处理逻辑
type ClientPeer struct {
	conn   *websocket.Conn
	client *db.Client
}
