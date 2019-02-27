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
	reconnectTimes = 10
	pingInterval   = time.Second * 3
)

const (
	peerTypeClient = 1
	peerTypeServer = 2
)

// ServerPeer 代表一个服务器节点，每个服务器节点都会建立与其它服务器节点的接连，
// 这个对象用于处理跨服务节点消息收发。
type ServerPeer struct {
	*peer.Peer
	hub            *Hub
	entity         *database.Server
	isOutServer    bool
	reconnectTimes int
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
	log.Printf("server %v disconnected ; from %v:%v", p.entity.ID, p.entity.IP, p.entity.Port)
	if !p.isOutServer { //如果不是连接出去服务
		p.hub.unregister <- &delPeer{peer: p}
		return nil
	}

	// 尝试重连
	for p.reconnectTimes < reconnectTimes {
		server, _ := p.hub.serverCache.GetServer(p.entity.ID)
		// 如果服务器列表中不存在，说明服务宕机了
		if server == nil {
			p.hub.unregister <- &delPeer{peer: p}
			return nil
		}
		p.reconnectTimes++
		if err := p.connect(); err == nil {
			// 重连成功
			return nil
		}

		time.Sleep(time.Second * 3)
	}

	p.hub.unregister <- &delPeer{peer: p}
	return nil
}

func (p *ServerPeer) connect() error {
	serverSelf := p.hub.ServerSelf

	// 生成加密摘要
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%v%v%v", serverSelf.ID, serverSelf.IP, serverSelf.Port))
	io.WriteString(h, p.hub.config.Server.Secret)
	digest := hex.EncodeToString(h.Sum(nil))

	header := http.Header{}
	header.Add("id", fmt.Sprint(serverSelf.ID))
	header.Add("ip", serverSelf.IP)
	header.Add("port", strconv.Itoa(serverSelf.Port))
	header.Add("digest", digest)

	server := p.entity
	u := url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", server.IP, server.Port), Path: "/server"}
	log.Printf("connecting to %s", u.String())

	dialar := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 3 * time.Second,
	}

	conn, _, err := dialar.Dial(u.String(), header)
	if err != nil {
		log.Println("dial:", err)
		return err
	}
	p.reconnectTimes = 0
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
				log.Printf("[%v]join group[%v]", p.entity.ID, group)
			case wire.GroupOut:
				if err = p.hub.groupCache.Leave(group, p.entity.ID); err != nil {
					st = wire.AckStateFail
				}
				log.Printf("[%v]join Leave[%v]", p.entity.ID, group)
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
	log.Printf("client %v disconnected", p.entity.ID)
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
	if err := serverPeer.connect(); err != nil {
		return nil, err
	}

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
	// clientPeers 缓存客户端节点数据
	clientPeers map[string]*ClientPeer
	// serverPeers 缓存服务端节点数据
	serverPeers map[uint64]*ServerPeer

	register   chan *addPeer
	unregister chan *delPeer

	sendMessage chan sendMessage
	saveMessage chan saveMessage

	quit chan struct{}
}

// NewHub 创建一个 Server 对象，并初始化
func NewHub(cfg *config.Config) (*Hub, error) {
	var upgrader = &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			if cfg.Server.Origin == "*" {
				return true
			}
			rOrigin := r.Header.Get("Origin")
			if strings.Contains(cfg.Server.Origin, rOrigin) {
				return true
			}
			log.Println("refuse", rOrigin)
			return false
		},
	}

	var clientCache database.ClientCache
	var serverCache database.ServerCache

	if cfg.Server.Mode == config.ModeSingle {
		clientCache = newHubClientCache(cfg.Cache.Client, false)
	} else {
		clientCache = newHubClientCache(cfg.Cache.Client, true)
		serverCache = newHubServerCache(cfg.Cache.Server, pingInterval)
	}

	hub := &Hub{
		upgrader:     upgrader,
		config:       cfg,
		ServerID:     cfg.Server.ID,
		clientCache:  clientCache,
		serverCache:  serverCache,
		groupCache:   cfg.Cache.Group,
		messageStore: cfg.MessageStore,
		clientPeers:  make(map[string]*ClientPeer, 1000),
		serverPeers:  make(map[uint64]*ServerPeer, 10),
		register:     make(chan *addPeer, 1),
		unregister:   make(chan *delPeer, 1),
		sendMessage:  make(chan sendMessage, 1),
		saveMessage:  make(chan saveMessage, 1),
		quit:         make(chan struct{}),
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
		listenIP := cfg.Server.Addr
		log.Println("listen on ", fmt.Sprintf("%s:%d", listenIP, cfg.Server.Listen))
		err := http.ListenAndServe(fmt.Sprintf("%s:%d", listenIP, cfg.Server.Listen), nil)

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
	if !checkDigest(hub.config.Server.Secret, fmt.Sprintf("%v%v", clientID, nonce), digest) {
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
	// 注册节点到服务器
	hub.register <- &addPeer{peer: clientPeer, done: nil}
	log.Printf("client %v connected from %v", clientID, r.RemoteAddr)
}

// 处理来自服务器节点的连接
func handleServerWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	ID, _ := strconv.ParseUint(r.Header.Get("id"), 0, 64)
	IP := r.Header.Get("ip")
	Port, _ := strconv.Atoi(r.Header.Get("port"))
	digest := r.Header.Get("digest")

	if ID == 0 || IP == "" || Port == 0 {
		// 错误处理，断开
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// 校验digest及数据完整性
	if !checkDigest(hub.config.Server.Secret, fmt.Sprintf("%v%v%v", ID, IP, Port), digest) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

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

	log.Printf("server %v connected from %v", ID, r.RemoteAddr)
}

func checkDigest(secret, text, digest string) bool {
	h := md5.New()
	io.WriteString(h, text)
	io.WriteString(h, secret)
	return digest == hex.EncodeToString(h.Sum(nil))
}

// Run start all handlers
func (h *Hub) Run() {

	go h.peerHandler()
	go h.messageHandler()
	go h.saveMessageHandler()
	go h.pingHandler()

	// 连接到其它服务器节点,并且对放开放服务
	h.outPeerHandler()

	<-h.quit
}

func (h *Hub) pingHandler() error {
	if h.config.Server.Mode == config.ModeSingle {
		return nil
	}
	ticker := time.NewTicker(time.Second * 3)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			server := h.ServerSelf
			server.ClientNum = len(h.clientPeers)
			h.serverCache.SetServer(server)
		}
	}
}

// 与其它服务器节点建立长连接
func (h *Hub) outPeerHandler() error {
	if h.config.Server.Mode == config.ModeSingle {
		return nil
	}
	log.Println("start outPeerhandler")
	servers, err := h.serverCache.GetServers()
	if err != nil {
		return err
	}
	serverSelf := database.Server{
		ID:         h.ServerID,
		IP:         wire.GetOutboundIP().String(),
		Port:       h.config.Server.Listen,
		StartAt:    time.Now().Unix(),
		ClientNum:  0,
		OutServers: make(map[uint64]string),
	}
	h.ServerSelf = &serverSelf

	// 主动连接到其它节点
	for _, server := range servers {
		if server.ID == serverSelf.ID {
			continue
		}
		serverPeer, err := newServerPeer(h, &server)
		if err != nil {
			continue
		}
		h.register <- &addPeer{peer: serverPeer, done: nil}
	}

	// 记录到远程缓存中
	h.serverCache.SetServer(&serverSelf)
	log.Println("end outPeerhandler")
	return nil
}

func (h *Hub) peerHandler() {
	log.Println("start peerHandler")
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
				h.ServerSelf.OutServers[peer.entity.ID] = time.Now().Format(time.Stamp)
			}
			if p.done != nil {
				p.done <- struct{}{}
			}
		case p := <-h.unregister:
			switch p.peer.(type) {
			case *ClientPeer:
				peer := p.peer.(*ClientPeer)
				if _, ok := h.clientPeers[peer.entity.ID]; ok {
					delete(h.clientPeers, peer.entity.ID)
					h.clientCache.DelClient(peer.entity.ID)
				}
			case *ServerPeer:
				peer := p.peer.(*ServerPeer)
				if _, ok := h.serverPeers[peer.entity.ID]; ok {
					delete(h.serverPeers, peer.entity.ID)
					delete(h.ServerSelf.OutServers, peer.entity.ID)
					peer.Close()
				}
			}
			if p.done != nil {
				p.done <- struct{}{}
			}
		}
	}
}

// 处理消息转发
func (h *Hub) messageHandler() {
	log.Println("start messageHandler")
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
				if err == nil && client != nil {
					// 消息转发过去
					if server, ok := h.serverPeers[client.ServerID]; ok {
						server.PushMessage(msg.message, nil)
					}
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
		}
	}
}

func (h *Hub) saveMessageHandler() {
	log.Println("start saveMessageHandler")
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
	if h.config.Server.Mode == config.ModeCluster {
		h.serverCache.DelServer(h.ServerID)
		log.Println("clean server in cache")
	}

	for _, peer := range h.clientPeers {
		peer.Close()
		h.clientCache.DelClient(peer.entity.ID)
	}

	for _, peer := range h.serverPeers {
		peer.Close()
	}

	time.Sleep(time.Second)
}

// ClientCache ClientCache wapper
type ClientCache struct {
	cache     database.ClientCache
	isCluster bool
}

func newHubClientCache(cache database.ClientCache, isCluster bool) *ClientCache {
	return &ClientCache{cache: cache, isCluster: isCluster}
}

// AddClient AddClient
func (c *ClientCache) AddClient(client *database.Client) error {
	if c.isCluster {
		return c.cache.AddClient(client)
	}
	return nil
}

// DelClient DelClient
func (c *ClientCache) DelClient(ID string) (int, error) {
	if c.isCluster {
		return c.cache.DelClient(ID)
	}
	return 0, nil
}

// GetClient GetClient
func (c *ClientCache) GetClient(ID string) (*database.Client, error) {
	if c.isCluster {
		return c.cache.GetClient(ID)
	}
	return nil, nil
}

// ServerCache ServerCache
type ServerCache struct {
	cache        database.ServerCache
	pingInterval time.Duration
}

func newHubServerCache(cache database.ServerCache, pingInterval time.Duration) *ServerCache {
	return &ServerCache{cache: cache, pingInterval: pingInterval}
}

// SetServer SetServer
func (c *ServerCache) SetServer(server *database.Server) error {
	server.Ping = time.Now().Unix()
	return c.cache.SetServer(server)
}

// GetServer GetServer
func (c *ServerCache) GetServer(ID uint64) (*database.Server, error) {
	server, err := c.cache.GetServer(ID)
	if err != nil {
		return nil, err
	}
	if time.Duration(time.Now().Unix()-server.Ping)*time.Second > c.pingInterval*3 {
		c.cache.DelServer(server.ID)
		return nil, nil
	}
	return server, nil
}

// DelServer DelServer
func (c *ServerCache) DelServer(ID uint64) error {
	return c.cache.DelServer(ID)
}

// GetServers GetServers
func (c *ServerCache) GetServers() ([]database.Server, error) {
	servers, err := c.cache.GetServers()
	if err != nil {
		return nil, err
	}
	aliveServers := make([]database.Server, 0)
	for _, server := range servers {
		if time.Duration(time.Now().Unix()-server.Ping)*time.Second > c.pingInterval*3 {
			c.cache.DelServer(server.ID)
			continue
		}
		aliveServers = append(aliveServers, server)
	}
	return servers, nil
}
