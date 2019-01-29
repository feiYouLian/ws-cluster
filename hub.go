package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/ws-cluster/wire"

	"github.com/ws-cluster/database"

	"github.com/gorilla/websocket"
)

const (
	clientFlag = byte(0)
	serverFlag = byte(1)
)

// Peer 代表一个节点，可以是 client,server peer
type Peer interface {
	From() uint8
	Send() chan []byte
}

// Hub 是一个中转中心，所有 clientPeer
type Hub struct {
	upgrader    *websocket.Upgrader
	config      *Config
	clientCache *database.ClientCache
	groupCache  *database.GroupCache
	serverCache *database.ServerCache

	clientPeers map[uint64]*ClientPeer
	serverPeers map[string]*ServerPeer

	registClient   chan *ClientPeer
	unregistClient chan *ClientPeer
	registServer   chan *ServerPeer
	unregistServer chan *ServerPeer

	message     chan []byte
	saveMessage chan wire.Message
}

// NewHub 创建一个 Server 对象，并初始化
func NewHub(config *Config) (*Hub, error) {

	var upgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	redis := database.InitRedis(config.RedisIP)

	hub := &Hub{
		upgrader:    upgrader,
		config:      config,
		clientCache: database.New,
	}

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
		ID:   uint64(id),
		Name: name,
	})

	if err != nil {
		log.Println(err)
		return
	}

	hub.registClient <- clientPeer

}

// 处理来自服务器节点的连接
func handleServerWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	ID := r.Header.Get("id")
	IP := r.Header.Get("ip")
	Port, _ := strconv.Atoi(r.Header.Get("port"))

	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	serverPeer := NewServerPeer(conn, &database.Server{
		ID:   ID,
		IP:   IP,
		Port: Port,
	})

	hub.registServer <- serverPeer
}

func (h *Hub) run() {
	// 连接到其它服务器节点
	h.initServer()

	go h.handlePeer()
	go h.handleMessage()

}

func (h *Hub) initServer() error {
	servers, err := h.serverCache.GetServers()
	if err != nil {
		return err
	}
	serverSelf := database.Server{
		ID:   h.config.ServerID,
		IP:   GetOutboundIP().String(),
		Port: h.config.ServerListen,
	}

	// 主动连接到其它节点
	for _, server := range servers {
		u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", server.IP, server.Port), Path: "/server"}
		log.Printf("connecting to %s", u.String())

		header := http.Header{}
		header.Add("id", serverSelf.ID)
		header.Add("ip", serverSelf.IP)
		header.Add("port", strconv.Itoa(serverSelf.Port))

		conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
		if err != nil {
			log.Fatal("dial:", err)
		}
		serverPeer := NewServerPeer(conn, &server)

		h.registServer <- serverPeer
	}

	// 记录到远程缓存中
	h.serverCache.AddServer(&serverSelf)

	return nil
}

func (h *Hub) handlePeer() {
	for {
		select {
		case peer := <-h.registClient:
			h.clientPeers[peer.client.ID] = peer

		case peer := <-h.unregistClient:
			if _, ok := h.clientPeers[peer.client.ID]; ok {
				delete(h.clientPeers, peer.client.ID)
				close(peer.send)
			}
		case peer := <-h.registServer:
			h.serverPeers[peer.server.ID] = peer

		case peer := <-h.unregistServer:
			h.handlerServerUnregister(peer)
		}
	}
}

func (h *Hub) handlerServerUnregister(peer *ServerPeer) {
	if _, ok := h.serverPeers[peer.server.ID]; ok {
		delete(h.serverPeers, peer.server.ID)
		close(peer.send)

		// 判断当前节点是否为主节点

		// 如果是主节点，维护服务器列表
	}

}

// 处理消息转发
func (h *Hub) handleMessage() {
	for {
		select {
		case msg := <-h.message:
			from := msg[0]
			message, err := wire.ReadMessage(bytes.NewReader(msg))
			if err != nil {
				fmt.Println(err)
				continue
			}
			if message.Msgtype() == wire.MsgTypeChat {
				if from == clientFlag {
					h.saveMessage <- message
				}

				chatMsg := message.(*wire.Msgchat)
				// 消息转发到其它服务器
				h.handleChatMsgForwardToServers(chatMsg)
				// 转发消息出去
				h.handleChatMsgForwardToClients(chatMsg)

			} else if message.Msgtype() == wire.MsgTypeChatAck {
				msg := message.(*wire.MsgchatAck)
				if client, ok := h.clientPeers[msg.To]; ok {
					client.send <- message
				} else {
					// 读取目标client所在的服务器
					client, _ := h.clentCache.GetClient(msg.To)
					// 消息转发过去
					if server, ok := h.serverPeers[client.ServerID]; ok {
						server.send <- message
					}
				}
			}

		}
	}
}

// 转发消息到其它服务器节点
func (h *Hub) handleChatMsgForwardToServers(chatMsg *wire.Msgchat) {
	if chatMsg.Type == wire.ChatTypeGroup {
		for _, serverpeer := range h.serverPeers {
			serverpeer.send <- chatMsg
		}
	} else if chatMsg.Type == wire.ChatTypeSingle {
		if _, ok := h.clientPeers[chatMsg.To]; !ok {
			// 读取目标client所在的服务器
			client, _ := h.clentCache.GetClient(chatMsg.To)
			// 消息转发过去
			if server, ok := h.serverPeers[client.ServerID]; ok {
				server.send <- chatMsg
			}
		}
	}
}

// 转发消息到客户端节点
func (h *Hub) handleChatMsgForwardToClients(chatMsg *wire.Msgchat) {
	if chatMsg.Type == wire.ChatTypeGroup {
		group, _ := h.groupCache.GetGroup(chatMsg.To)
		for _, to := range group.Clients {
			if client, ok := h.clientPeers[to]; ok {
				client.send <- chatMsg
			}
		}
	} else if chatMsg.Type == wire.ChatTypeSingle {
		if clientPeer, ok := h.clientPeers[chatMsg.To]; ok {
			clientPeer.send <- chatMsg
		}
	}
}
