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
	"strings"
	"time"

	"github.com/ws-cluster/config"

	"github.com/ws-cluster/peer"
	"github.com/ws-cluster/wire"

	"github.com/ws-cluster/database"

	"github.com/gorilla/websocket"
)

const (
	clientFlag     = byte(0)
	serverFlag     = byte(1)
	reconnectTimes = 180
)

const (
	peerTypeClient = 1
	peerTypeServer = 2
)

// ServerPeer 代表一个服务器节点，每个服务器节点都会建立与其它服务器节点的接连，
// 这个对象用于处理跨服务节点消息收发。
type ServerPeer struct {
	*peer.Peer
	hub         *Hub
	entity      *database.Server
	isOutServer bool
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
	log.Println("server disconnected", p.entity.ID)

	done := make(chan struct{})
	p.hub.unregister <- &delPeer{peer: p, done: done}
	<-done

	// 如果是出去的服务，就尝试重连
	if p.isOutServer {
		for i := 0; i < reconnectTimes; i++ {
			time.Sleep(time.Second * 1)
			if err := p.connect(); err == nil {
				break
			}
		}
	}

	return nil
}

func (p *ServerPeer) connect() error {
	header := http.Header{}
	header.Add("id", fmt.Sprint(p.hub.ServerSelf.ID))
	header.Add("ip", p.hub.ServerSelf.IP)
	header.Add("port", strconv.Itoa(p.hub.ServerSelf.Port))

	server := p.entity
	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", server.IP, server.Port), Path: "/server"}
	log.Printf("connecting to %s", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		log.Println("dial:", u.String(), err)
		return err
	}
	p.SetConnection(conn)
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
	msg, err := wire.ReadMessage(bytes.NewReader(message))
	if err != nil {
		return err
	}
	st := wire.AckStateSent

	// 处理消息逻辑
	switch msg.Header().Msgtype {
	case wire.MsgTypeChat:
		// 保存消息到 db
		err = p.saveMessage(msg.(*wire.Msgchat))
		if err != nil {
			log.Println(err)
			st = wire.AckStateFail
		}
	case wire.MsgTypeGroupInOut:
		msgGroup := msg.(*wire.MsgGroupInOut)
		for _, group := range msgGroup.Groups {
			switch msgGroup.InOut {
			case wire.GroupIn:
				if err = p.hub.groupCache.Join(group, p.entity.ID); err != nil {
					st = wire.AckStateFail
				}
			case wire.GroupOut:
				if err = p.hub.groupCache.Leave(group, p.entity.ID); err != nil {
					st = wire.AckStateFail
				}
			}
		}
	}

	if msg.Header().Scope != wire.ScopeNull && st != wire.AckStateFail {
		log.Println("message forward to hub", msg.Header())
		// 消息转发
		p.hub.sendMessage <- sendMessage{from: clientFlag, message: message, header: msg.Header()}
	}

	// message ack
	ackmsg, _ := wire.MakeAckMessage(msg.Header().ID, st)
	p.PushMessage(ackmsg, nil)
	return nil
}

func (p *ClientPeer) saveMessage(chatMsg *wire.Msgchat) error {
	done := make(chan bool)
	chatmsg := &database.ChatMsg{
		From:     p.entity.ID,
		To:       chatMsg.Header().To,
		Scope:    chatMsg.Header().Scope,
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
	log.Println("client disconnected", p.entity.ID)
	p.hub.unregister <- &delPeer{peer: p, done: nil}
	return nil
}

// newServerPeer 主动去连接另一台服务节点器
func newServerPeer(h *Hub, server *database.Server) (*ServerPeer, error) {

	serverPeer := &ServerPeer{
		hub:         h,
		entity:      server,
		isOutServer: true,
	}

	peer := peer.NewPeer(fmt.Sprintf("S%v", server.ID),
		&peer.Config{
			Listeners: &peer.MessageListeners{
				OnMessage:    serverPeer.OnMessage,
				OnDisconnect: serverPeer.OnDisconnect,
			},
			MaxMessageSize: h.config.Peer.MaxMessageSize,
		})

	serverPeer.Peer = peer
	// 主动进行连接
	serverPeer.connect()

	return serverPeer, nil
}

// bindServerPeer 处理其它服务器节点过来的连接
func bindServerPeer(h *Hub, conn *websocket.Conn, server *database.Server) (*ServerPeer, error) {

	serverPeer := &ServerPeer{
		hub:         h,
		entity:      server,
		isOutServer: false,
	}

	peer := peer.NewPeer(fmt.Sprintf("S%v", server.ID),
		&peer.Config{
			Listeners: &peer.MessageListeners{
				OnMessage:    serverPeer.OnMessage,
				OnDisconnect: serverPeer.OnDisconnect,
			},
			MaxMessageSize: h.config.Peer.MaxMessageSize,
		})

	serverPeer.Peer = peer
	// 被动接受的连接
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
		MaxMessageSize: h.config.Peer.MaxMessageSize,
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

type addPeer struct {
	peer interface{}
	done chan struct{}
}

type delPeer struct {
	peer interface{}
	done chan struct{}
}

// Hub 是一个服务中心，所有 clientPeer
type Hub struct {
	upgrader    *websocket.Upgrader
	config      *config.Config
	ServerID    uint64
	ServerSelf  *database.Server
	clientCache database.ClientCache
	groupCache  database.GroupCache
	serverCache database.ServerCache

	messageStore database.MessageStore

	clientPeers map[string]*ClientPeer
	serverPeers map[uint64]*ServerPeer

	register   chan *addPeer
	unregister chan *delPeer

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
			if config.Server.Origin == "*" {
				return true
			}
			rOrigin := r.Header.Get("Origin")
			if strings.Contains(config.Server.Origin, rOrigin) {
				return true
			}
			log.Println("refuse", rOrigin)
			return false
		},
	}
	// build a client instance of redis

	redis := database.InitRedis(config.Redis.IP, config.Redis.Port, config.Redis.Password)

	mysqldb := database.InitDb(config.Mysql.IP, config.Mysql.Port, config.Mysql.User, config.Mysql.Password, config.Mysql.DbName)

	hub := &Hub{
		upgrader:     upgrader,
		config:       config,
		ServerID:     config.Server.ID,
		clientCache:  database.NewRedisClientCache(redis),
		serverCache:  database.NewRedisServerCache(redis),
		groupCache:   database.NewMemGroupCache(),
		messageStore: database.NewMysqlMessageStore(mysqldb),
		clientPeers:  make(map[string]*ClientPeer, 100),
		serverPeers:  make(map[uint64]*ServerPeer, 100),
		register:     make(chan *addPeer, 1),
		unregister:   make(chan *delPeer, 1),
		sendMessage:  make(chan sendMessage, 1),
		saveMessage:  make(chan saveMessage, 1),
		quit:         make(chan struct{}),
	}

	// clean all of old clients due to crash
	hub.clientCache.DelAll(config.Server.ID)

	// regist a service for client
	http.HandleFunc("/client", func(w http.ResponseWriter, r *http.Request) {
		handleClientWebSocket(hub, w, r)
	})
	// regist a service for server
	http.HandleFunc("/server", func(w http.ResponseWriter, r *http.Request) {
		handleServerWebSocket(hub, w, r)
	})
	go func() {
		listenIP := config.Server.Addr
		log.Println("listen on ", fmt.Sprintf("%s:%d", listenIP, config.Server.Listen))
		err := http.ListenAndServe(fmt.Sprintf("%s:%d", listenIP, config.Server.Listen), nil)

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
		ID:       clientID,
		ServerID: hub.ServerID,
		LoginAt:  uint32(time.Now().Unix()),
	})

	if err != nil {
		log.Println(err)
		return
	}

	hub.register <- &addPeer{peer: clientPeer, done: nil}
	log.Printf("client in ,%v, clientid: %v", r.RemoteAddr, clientID)
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
	serverPeer, err := bindServerPeer(hub, conn, &database.Server{
		ID:   ID,
		IP:   IP,
		Port: Port,
	})
	if err != nil {
		log.Println(err)
		return
	}

	hub.register <- &addPeer{peer: serverPeer, done: nil}
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
		ID:       h.ServerID,
		IP:       wire.GetOutboundIP().String(),
		Port:     h.config.Server.Listen,
		BootTime: time.Now(),
	}
	h.ServerSelf = &serverSelf

	// 主动连接到其它节点
	for _, server := range servers {
		if server.ID == serverSelf.ID {
			continue
		}
		serverPeer, err := newServerPeer(h, &server)
		if err != nil {
			log.Println(err)
			continue
		}
		h.register <- &addPeer{peer: serverPeer, done: nil}
	}

	// 记录到远程缓存中
	h.serverCache.AddServer(&serverSelf)

	return nil
}

func (h *Hub) handlePeer() {
	for {
		select {
		case p := <-h.register:
			switch p.peer.(type) {
			case *ClientPeer:
				peer := p.peer.(*ClientPeer)
				if p, ok := h.clientPeers[peer.entity.ID]; ok {
					// 如果节点已经登陆，就把前一个 peer踢下线
					p.Close()
				} else {
					client, err := h.clientCache.GetClient(peer.entity.ID)
					// 如果节点已经登陆到其它服务器，就发送一个MsgKillClient踢去另外一台服务上的连接
					if err == nil && client != nil {

					}
				}
				h.clientPeers[peer.entity.ID] = peer
				h.clientCache.AddClient(peer.entity)
			case *ServerPeer:
				peer := p.peer.(*ServerPeer)
				h.serverPeers[peer.entity.ID] = peer
			}
		case p := <-h.unregister:
			switch p.peer.(type) {
			case *ClientPeer:
				peer := p.peer.(*ClientPeer)
				if _, ok := h.clientPeers[peer.entity.ID]; ok {
					delete(h.clientPeers, peer.entity.ID)
					peer.Close()

					h.clientCache.DelClient(peer.entity.ID, peer.entity.ServerID)
				}

			case *ServerPeer:
				peer := p.peer.(*ServerPeer)
				if _, ok := h.serverPeers[peer.entity.ID]; ok {
					delete(h.serverPeers, peer.entity.ID)
					peer.Close()
				}
			}
		case <-h.quit:
			break
		}
	}
}

// func isMasterServer(servers []database.Server, ID, exID uint64) bool {
// 	isMaster := true
// 	for _, s := range servers {
// 		if s.ID == exID {
// 			continue
// 		}
// 		if s.ID < ID { // 最小的 id 就是存活最久的服务，就是主服务
// 			isMaster = false
// 		}
// 	}
// 	return isMaster
// }

// 处理消息转发
func (h *Hub) handleMessage() {
	for {
		select {
		case msg := <-h.sendMessage:
			header := msg.header
			if header == nil {
				return
			}
			if header.Scope == wire.ScopeClient {
				to := header.To
				// 在当前服务器节点中找到了目标客户端
				if client, ok := h.clientPeers[to]; ok {
					client.PushMessage(msg.message, nil)
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
					server.PushMessage(msg.message, nil)
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
				log.Println("send group message to ", clients)
				for _, clientID := range clients {
					if client, ok := h.clientPeers[clientID]; ok {
						client.PushMessage(msg.message, nil)
					} else {
						// 如果发现用户不存在就清理掉
						h.groupCache.Leave(group, clientID)
					}
				}
			}
		case <-h.quit:
			break
		}
	}
}

func (h *Hub) handlesaveMessage() {
	for {
		select {
		case msg := <-h.saveMessage:
			log.Println("save message", msg.msg.From, msg.msg.Text)
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
	h.clean()

	h.quit <- struct{}{}
}

// clean clean hub
func (h *Hub) clean() {
	for _, peer := range h.clientPeers {
		peer.Close()
		h.clientCache.DelClient(peer.entity.ID, peer.entity.ServerID)
	}
	log.Println("clean clients in cache")
	h.serverCache.DelServer(h.ServerID)
	log.Println("clean server in cache")
}
