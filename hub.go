package main

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// Hub 是一个中转中心，所有 clientPeer
type Hub struct {
	upgrader    *websocket.Upgrader
	config      Config
	clientPeers map[int64]*ClientPeer
	serverPeer  map[string]*ServerPeer

	// Register requests from the clients.
	register chan *ClientPeer

	// Unregister requests from clients.
	unregister chan *ServerPeer
}

// NewHub 创建一个 Server 对象，并初始化
func NewHub(config *Config) (*Hub, error) {

	var upgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	hub := &Hub{upgrader: upgrader}

	// http.HandleFunc("/", serveHome)
	http.HandleFunc("/client", func(w http.ResponseWriter, r *http.Request) {
		handleClientWebSocket(hub, w, r)
	})
	http.HandleFunc("/server", func(w http.ResponseWriter, r *http.Request) {
		handleServerWebSocket(hub, w, r)
	})

	err := http.ListenAndServe(config.ServerAddr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
		return nil, err
	}

	return hub, nil
}

// 处理来自客户端节点的连接
func handleClientWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
}

// 处理来自服务器节点的连接
func handleServerWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {

}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:

		case client := <-h.unregister:
		}
	}
}

func (h *Hub) handleClientMsg() {

}
