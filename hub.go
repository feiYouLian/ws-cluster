package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ws-cluster/peer"
	"github.com/ws-cluster/wire"

	"github.com/ws-cluster/database"

	"github.com/gorilla/websocket"
)

const (
	clientFlag = byte(0)
	serverFlag = byte(1)
)

// ServerPeer 代表一个服务器节点，每个服务器节点都会建立与其它服务器节点的接连，
// 这个对象用于处理跨服务节点消息收发。
type ServerPeer struct {
	*peer.Peer
	hub    *Hub
	entity *database.Server
}

// OnMessage 接收消息
func (p *ServerPeer) OnMessage(message []byte) error {
	header, err := wire.ReadHeader(bytes.NewReader(message))
	if err != nil {
		return err
	}
	if header.Scope != wire.ScopeNull {
		p.hub.sendMessage <- sendMessage{from: serverFlag, message: message, header: header}
	}
	return nil
}

// OnDisconnect 对方断开接连
func (p *ServerPeer) OnDisconnect() error {
	p.hub.unregistServer <- p

	// 判断当前节点是否为主节点

	// 如果是主节点，维护服务器列表

	return nil
}

// ClientPeer 代表一个客户端节点，消息收发的处理逻辑
type ClientPeer struct {
	*peer.Peer
	hub    *Hub
	entity *database.Client
}

// OnMessage 接收消息
func (p *ClientPeer) OnMessage(message []byte) error {
	header, err := wire.ReadHeader(bytes.NewReader(message))
	if err != nil {
		return err
	}
	if header.Scope != wire.ScopeNull {
		// 如果是单聊消息，就保存到 db 中
		if header.Scope == wire.ScopeChat {
			msg, err := wire.ReadMessage(bytes.NewBuffer(message))
			if err != nil {
				return err
			}
			chatMsg, _ := msg.(*wire.Msgchat)

			done := make(chan bool)
			to, _ := header.Uint64To()
			chatmsg := &database.ChatMsg{
				From:     p.entity.ID,
				To:       to,
				Type:     chatMsg.Type,
				Text:     chatMsg.Text,
				CreateAt: time.Now(),
			}
			p.hub.saveMessage <- saveMessage{chatmsg, done}
			saved := <-done
			// message is saved
			if saved {
				ackHeader := &wire.MessageHeader{ID: header.ID, Msgtype: wire.MsgTypeChatAck, Scope: wire.ScopeChat}
				ackMessage, _ := wire.MakeEmptyMessage(ackHeader)
				msgChatAck, _ := ackMessage.(*wire.MsgchatAck)
				buf := &bytes.Buffer{}
				err := wire.WriteMessage(buf, msgChatAck)
				if err != nil {
					return err
				}
				p.PushMessage(buf.Bytes(), nil)
			}
		}
		p.hub.sendMessage <- sendMessage{from: clientFlag, message: message, header: header}
		return nil
	}
	msg, _ := wire.ReadMessage(bytes.NewReader(message))

	switch header.Msgtype {
	case wire.MsgTypeJoinGroup:
		msgJoinGroup := msg.(*wire.MsgJoinGroup)
		for _, group := range msgJoinGroup.Groups {
			p.hub.groupCache.Join(group, p.entity.ID)
		}
	case wire.MsgTypeLeaveGroup:
		msgLeaveGroup := msg.(*wire.MsgLeaveGroup)
		for _, group := range msgLeaveGroup.Groups {
			p.hub.groupCache.Leave(group, p.entity.ID)
		}
	}

	return nil
}

// OnDisconnect 接连断开
func (p *ClientPeer) OnDisconnect() error {
	p.hub.unregistClient <- p
	return nil
}

func newServerPeer(h *Hub, conn *websocket.Conn, server *database.Server) (*ServerPeer, error) {
	serverPeer := &ServerPeer{
		hub:    h,
		entity: server,
	}

	peer := peer.NewPeer(fmt.Sprintf("S%d", server.ID),
		&peer.Config{
			Listeners: &peer.MessageListeners{
				OnMessage:    serverPeer.OnMessage,
				OnDisconnect: serverPeer.OnDisconnect,
			},
			MessageQueueLen: 50,
		})

	serverPeer.Peer = peer
	serverPeer.SetConnection(conn)

	return serverPeer, nil
}

func newClientPeer(h *Hub, conn *websocket.Conn, client *database.Client) (*ClientPeer, error) {
	clientPeer := &ClientPeer{
		hub:    h,
		entity: client,
	}

	peer := peer.NewPeer(fmt.Sprintf("C%d", client.ID), &peer.Config{
		Listeners: &peer.MessageListeners{
			OnMessage:    clientPeer.OnMessage,
			OnDisconnect: clientPeer.OnDisconnect,
		},
		MessageQueueLen: h.config.Message.QueueLen,
	})

	clientPeer.Peer = peer
	clientPeer.SetConnection(conn)

	return clientPeer, nil
}

type sendMessage struct {
	from    byte
	header  *wire.MessageHeader
	message []byte
}

type saveMessage struct {
	msg  *database.ChatMsg
	done chan bool //
}

// Hub 是一个服务中心，所有 clientPeer
type Hub struct {
	upgrader    *websocket.Upgrader
	config      *Config
	clientCache database.ClientCache
	groupCache  database.GroupCache
	serverCache database.ServerCache

	messageStore database.MessageStore

	clientPeers map[uint64]*ClientPeer
	serverPeers map[uint64]*ServerPeer

	registClient   chan *ClientPeer
	unregistClient chan *ClientPeer
	registServer   chan *ServerPeer
	unregistServer chan *ServerPeer

	sendMessage chan sendMessage
	saveMessage chan saveMessage

	quit chan struct{}
}

// NewHub 创建一个 Server 对象，并初始化
func NewHub(config *Config) (*Hub, error) {

	var upgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	// build a client instance of redis

	redis := database.InitRedis(config.Redis.IP, config.Redis.Port, config.Redis.Password)

	mysqldb := database.InitDb(config.Mysql.IP, config.Mysql.Port, config.Mysql.User, config.Mysql.Password, config.Mysql.DbName)

	hub := &Hub{
		upgrader:       upgrader,
		config:         config,
		clientCache:    database.NewRedisClientCache(redis),
		serverCache:    database.NewRedisServerCache(redis),
		groupCache:     database.NewMemGroupCache(),
		messageStore:   database.NewMysqlMessageStore(mysqldb),
		registClient:   make(chan *ClientPeer, 1),
		unregistClient: make(chan *ClientPeer, 1),
		registServer:   make(chan *ServerPeer, 1),
		unregistServer: make(chan *ServerPeer, 1),
		sendMessage:    make(chan sendMessage, 1),
		saveMessage:    make(chan saveMessage, 1),
		quit:           make(chan struct{}),
	}

	// regist a service for client
	http.HandleFunc("/client", func(w http.ResponseWriter, r *http.Request) {
		handleClientWebSocket(hub, w, r)
	})
	// regist a service for server
	http.HandleFunc("/server", func(w http.ResponseWriter, r *http.Request) {
		handleServerWebSocket(hub, w, r)
	})

	err := http.ListenAndServe(config.Server.Addr, nil)
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
	clientPeer, err := newClientPeer(hub, conn, &database.Client{
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
	ID, _ := strconv.ParseUint(r.Header.Get("id"), 0, 64)
	IP := r.Header.Get("ip")
	Port, _ := strconv.Atoi(r.Header.Get("port"))

	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	serverPeer, err := newServerPeer(hub, conn, &database.Server{
		ID:   ID,
		IP:   IP,
		Port: Port,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	hub.registServer <- serverPeer
}

func (h *Hub) run() {
	// 连接到其它服务器节点
	h.initServer()

	go h.handlePeer()
	go h.handleMessage()
	go h.handlesaveMessage()
}

func (h *Hub) initServer() error {
	servers, err := h.serverCache.GetServers()
	if err != nil {
		return err
	}
	serverSelf := database.Server{
		ID:   h.config.Server.ID,
		IP:   GetOutboundIP().String(),
		Port: h.config.Server.Listen,
	}

	// 主动连接到其它节点
	for _, server := range servers {
		u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", server.IP, server.Port), Path: "/server"}
		log.Printf("connecting to %s", u.String())

		header := http.Header{}
		header.Add("id", fmt.Sprint(serverSelf.ID))
		header.Add("ip", serverSelf.IP)
		header.Add("port", strconv.Itoa(serverSelf.Port))

		conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
		if err != nil {
			log.Fatal("dial:", err)
		}
		serverPeer, err := newServerPeer(h, conn, &server)
		if err != nil {
			fmt.Println(err)
			continue
		}
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
			h.clientPeers[peer.entity.ID] = peer

		case peer := <-h.unregistClient:
			if _, ok := h.clientPeers[peer.entity.ID]; ok {
				delete(h.clientPeers, peer.entity.ID)
				peer.Close()

				h.clientCache.DelClient(peer.entity.ID, peer.entity.ServerID)
			}
		case peer := <-h.registServer:
			h.serverPeers[peer.entity.ID] = peer

		case peer := <-h.unregistServer:
			if _, ok := h.serverPeers[peer.entity.ID]; ok {
				delete(h.serverPeers, peer.entity.ID)
				peer.Close()
				// 不删除缓存。只有主服务才有权删除
			}
		}
	}
}

// 处理消息转发
func (h *Hub) handleMessage() {
	for {
		select {
		case msg := <-h.sendMessage:
			header := msg.header
			if header == nil {
				return
			}
			if header.Scope == wire.ScopeChat {
				to, _ := header.Uint64To()
				done := make(chan<- struct{})
				if client, ok := h.clientPeers[to]; ok {
					client.PushMessage(msg.message, done)
				} else {
					// 读取目标client所在的服务器
					client, _ := h.clientCache.GetClient(to)
					// 消息转发过去
					if server, ok := h.serverPeers[client.ServerID]; ok {
						server.PushMessage(msg.message, done)
					}
				}
			} else if header.Scope == wire.ScopeGroup {
				group, _ := header.StringTo()
				// 如果消息是直接来源于 client。就转发到其它服务器
				if msg.from == clientFlag {
					for _, serverpeer := range h.serverPeers {
						serverpeer.PushMessage(msg.message, nil)
					}
				}
				// 读取群用户列表。转发
				clients, _ := h.groupCache.GetGroupMembers(group)
				for _, clientID := range clients {
					if client, ok := h.clientPeers[clientID]; ok {
						client.PushMessage(msg.message, nil)
					} else {
						// 如果发现用户不存在就清理掉
						h.groupCache.Leave(group, clientID)
					}
				}
			}
		}
	}
}

func (h *Hub) handlesaveMessage() {
	for {
		select {
		case msg := <-h.saveMessage:
			fmt.Println(msg.msg.Text)
			err := h.messageStore.Save(msg.msg)
			if err != nil {
				msg.done <- false
			} else {
				msg.done <- true
			}
		}
	}
}
