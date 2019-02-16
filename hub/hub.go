package hub

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ws-cluster/config"

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
		sendMsg := sendMessage{from: clientFlag, message: message, header: header}
		err = p.saveMessage(&sendMsg)
		st := wire.AckStateSent
		if err != nil {
			st = wire.AckStateFail
			return err
		}
		// message ack
		ackmsg, _ := wire.MakeAckMessage(header.ID, st)
		p.PushMessage(ackmsg, nil)

		p.hub.sendMessage <- sendMsg
	}
	msg, _ := wire.ReadMessage(bytes.NewReader(message))

	switch header.Msgtype {
	case wire.MsgTypeGroupInOut:
		msgGroup := msg.(*wire.MsgGroupInOut)
		for _, group := range msgGroup.Groups {
			switch msgGroup.InOut {
			case wire.GroupIn:
				p.hub.groupCache.Join(group, p.entity.ID)
			case wire.GroupOut:
				p.hub.groupCache.Leave(group, p.entity.ID)
			}
		}
	}

	return nil
}

func (p *ClientPeer) saveMessage(sendMsg *sendMessage) error {
	header := sendMsg.header
	message := sendMsg.message
	// 如果是单聊消息，就保存到 db 中
	msg, err := wire.ReadMessage(bytes.NewBuffer(message))
	if err != nil {
		return err
	}
	chatMsg, _ := msg.(*wire.Msgchat)

	done := make(chan bool)
	chatmsg := &database.ChatMsg{
		From:     p.entity.ID,
		To:       header.To,
		Scope:    header.Scope,
		Type:     chatMsg.Type,
		Text:     chatMsg.Text,
		CreateAt: time.Now(),
	}
	p.hub.saveMessage <- saveMessage{chatmsg, done}
	// wait
	saved := <-done
	if !saved {
		return fmt.Errorf("message saved failed")
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

	peer := peer.NewPeer(fmt.Sprintf("S%v", server.ID),
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

	peer := peer.NewPeer(fmt.Sprintf("C%v", client.ID), &peer.Config{
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
	config      *config.Config
	clientCache database.ClientCache
	groupCache  database.GroupCache
	serverCache database.ServerCache

	messageStore database.MessageStore

	clientPeers map[string]*ClientPeer
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
func NewHub(config *config.Config) (*Hub, error) {

	var upgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
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
		clientPeers:    make(map[string]*ClientPeer, 100),
		serverPeers:    make(map[uint64]*ServerPeer, 100),
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
	go func() {
		log.Println("listen on ", config.Server.Listen)
		err := http.ListenAndServe(fmt.Sprintf(":%d", config.Server.Listen), nil)

		if err != nil {
			log.Println("ListenAndServe: ", err)
			return
		}
	}()
	return hub, nil
}

// 处理来自客户端节点的连接
func handleClientWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("id")
	nonce := q.Get("nonce")
	digest := q.Get("digest")

	if clientID == "" || nonce == "" || digest == "" {
		// 错误处理，断开
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// 校验digest及数据完整性
	h := md5.New()
	io.WriteString(h, clientID)
	io.WriteString(h, nonce)
	io.WriteString(h, hub.config.Server.Secret)
	if digest != hex.EncodeToString(h.Sum(nil)) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// upgrade
	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	clientPeer, err := newClientPeer(hub, conn, &database.Client{
		ID: clientID,
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
		log.Println(err)
		return
	}

	hub.registServer <- serverPeer
}

// Run start all handlers
func (h *Hub) Run() {

	// 连接到其它服务器节点
	h.handleOutbind()

	go h.handlePeer()
	go h.handleMessage()
	go h.handlesaveMessage()
	<-h.quit
}

func (h *Hub) handleOutbind() error {
	servers, err := h.serverCache.GetServers()
	if err != nil {
		return err
	}
	serverSelf := database.Server{
		ID:   h.config.Server.ID,
		IP:   wire.GetOutboundIP().String(),
		Port: h.config.Server.Listen,
	}

	// 主动连接到其它节点
	for _, server := range servers {
		if server.ID == serverSelf.ID {
			continue
		}
		u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", server.IP, server.Port), Path: "/server"}
		log.Printf("connecting to %s", u.String())

		header := http.Header{}
		header.Add("id", fmt.Sprint(serverSelf.ID))
		header.Add("ip", serverSelf.IP)
		header.Add("port", strconv.Itoa(serverSelf.Port))

		conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
		if err != nil {
			log.Println("dial:", err)
			continue
		}
		serverPeer, err := newServerPeer(h, conn, &server)
		if err != nil {
			log.Println(err)
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
				to := header.To
				done := make(chan<- struct{})
				// 在当前服务器节点中找到了目标客户端
				if client, ok := h.clientPeers[to]; ok {
					client.PushMessage(msg.message, done)
					continue
				}

				// 读取目标client所在的服务器
				client, err := h.clientCache.GetClient(to)

				// 如果读不到数据，就广播消息
				if err != nil {
					for _, serverpeer := range h.serverPeers {
						serverpeer.PushMessage(msg.message, nil)
					}
					continue
				}

				// 消息转发过去
				if server, ok := h.serverPeers[client.ServerID]; ok {
					server.PushMessage(msg.message, done)
				}

			} else if header.Scope == wire.ScopeGroup {
				group := header.To
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
			log.Println(msg.msg.Text)
			err := h.messageStore.Save(msg.msg)
			if err != nil {
				msg.done <- false
			} else {
				msg.done <- true
			}
		}
	}
}

// Close close hub
func (h *Hub) Close() {
	h.quit <- struct{}{}
}
