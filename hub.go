package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/ws-cluster/database"

	"github.com/gorilla/websocket"
)

// Hub 是一个中转中心，所有 clientPeer
type Hub struct {
	upgrader    *websocket.Upgrader
	config      Config
	clientPeers map[int64]*ClientPeer
	serverPeer  map[string]*ServerPeer

	// Register requests from the clients.
	clientRegister chan *ClientPeer
	// Unregister requests from clients.
	clientUnregister chan *ClientPeer

	serverRegister   chan *ServerPeer
	serverUnregister chan *ServerPeer

	msgFromClient chan []byte
	msgFromServer chan []byte
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
	q := r.URL.Query()
	clientID := q.Get("id")
	name := q.Get("name")
	nonce := q.Get("nonce")
	digest := q.Get("digest")

	if clientID == "" || name == "" || nonce == "" || digest == "" {
		// 错误处理，断开
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// 校验digest及数据完整性

	// upgrade
	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	id, _ := strconv.ParseInt(clientID, 0, 64)
	clientPeer, err := NewClientPeer(conn, &database.Client{
		ID:   id,
		Name: name,
	})

	if err != nil {
		log.Println(err)
		return
	}

	hub.clientRegister <- clientPeer

}

// 处理来自服务器节点的连接
func handleServerWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {

}

func (h *Hub) run() {
	go h.handleClientMsg()
	go h.handleServerMsg()
}

func (h *Hub) handleClientMsg() {
	for {
		select {
		case client := <-h.clientRegister:

		case client := <-h.clientUnregister:

		case msg := <-h.msgFromClient:

		}
	}
}

func (h *Hub) handleServerMsg() {
	for {
		select {
		case serverPeer := <-h.serverRegister:

		case serverPeer := <-h.serverUnregister:

		case msg := <-h.msgFromServer:

		}
	}
}
