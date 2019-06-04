package hub

import (
	"bytes"
	"container/list"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ws-cluster/filelog"

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
		p.hub.msgQueue <- &Msg{from: serverFlag, message: message}
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
	header, err := wire.ReadHeader(bytes.NewReader(message))
	if err != nil {
		return err
	}
	st := wire.AckStateSent

	// 处理消息逻辑
	switch header.Msgtype {
	case wire.MsgTypeGroupInOut:
		msg, err := wire.ReadMessage(bytes.NewReader(message))
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

	if header.Scope != wire.ScopeNull && st != wire.AckStateFail {
		log.Println("message forward to hub", header)
		// 消息转发
		p.hub.msgQueue <- &Msg{from: clientFlag, message: message}
	}

	// message ack
	ackmsg, _ := wire.MakeAckMessage(header.ID, st)
	p.PushMessage(ackmsg, nil)
	return nil
}

// OnDisconnect 接连断开
func (p *ClientPeer) OnDisconnect() error {
	log.Printf("client %v disconnect", p.Peer.ID)
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

	peer := peer.NewPeer(fmt.Sprintf("C%v_%v", client.ID, time.Now().UnixNano()), &peer.Config{
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

// Msg to hub
type Msg struct {
	from    byte
	message []byte
}

// type saveMessage struct {
// 	msg  *database.ChatMsg
// 	done chan bool //
// }

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

	// clientPeers 缓存客户端节点数据
	clientPeers map[string]*ClientPeer
	// serverPeers 缓存服务端节点数据
	serverPeers map[uint64]*ServerPeer

	messageLog *filelog.FileLog
	loginLog   *filelog.FileLog

	register   chan *addPeer
	unregister chan *delPeer

	msgRelay     chan *Msg
	msgQueue     chan *Msg
	msgRelayDone chan uint32
	quit         chan struct{}
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

	if cfg.Server.Mode == config.ModeSingle {
		clientCache = newHubClientCache(cfg.Cache.Client, false)
	} else {
		clientCache = newHubClientCache(cfg.Cache.Client, true)
	}
	messageLogConfig := &filelog.Config{
		File: cfg.Server.MessageFile,
		SubFunc: func(msgs []*bytes.Buffer) error {
			return saveMessagesToDb(cfg.MessageStore, msgs)
		},
	}
	messageLog, err := filelog.NewFileLog(messageLogConfig)
	if err != nil {
		return nil, err
	}

	hub := &Hub{
		upgrader:     upgrader,
		config:       cfg,
		ServerID:     cfg.Server.ID,
		clientCache:  clientCache,
		serverCache:  cfg.Cache.Server,
		groupCache:   cfg.Cache.Group,
		clientPeers:  make(map[string]*ClientPeer, 1000),
		serverPeers:  make(map[uint64]*ServerPeer, 10),
		register:     make(chan *addPeer, 1),
		unregister:   make(chan *delPeer, 1),
		msgQueue:     make(chan *Msg, 1),
		msgRelay:     make(chan *Msg, 1),
		msgRelayDone: make(chan uint32, 1),

		messageLog: messageLog,
		quit:       make(chan struct{}),
	}

	// regist a service for client
	http.HandleFunc("/client", func(w http.ResponseWriter, r *http.Request) {
		handleClientWebSocket(hub, w, r)
	})
	// regist a service for server
	http.HandleFunc("/server", func(w http.ResponseWriter, r *http.Request) {
		handleServerWebSocket(hub, w, r)
	})

	http.HandleFunc("/msg/send", func(w http.ResponseWriter, r *http.Request) {
		httpSendMsgHandler(hub, w, r)
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
	log.Printf("client %v connecting from %v", clientID, r.RemoteAddr)
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

	log.Printf("server %v connecting from %v", ID, r.RemoteAddr)
}

// 处理 http 过来的消息发送
func httpSendMsgHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	var body database.ChatMsg
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// fmt.Println("httpSendMsg ", r.RemoteAddr, body.To, body.Text)

	header := &wire.MessageHeader{
		ID:      uint32(time.Now().Unix()),
		Msgtype: wire.MsgTypeChat,
		Scope:   body.Scope,
		To:      body.To,
	}
	msg, err := wire.MakeEmptyMessage(header)
	if err != nil {
		fmt.Fprint(w, err.Error())
		return
	}
	msgchat := msg.(*wire.Msgchat)
	msgchat.Text = body.Text
	msgchat.Type = body.Type
	msgchat.Extra = body.Extra
	msgchat.From = body.From
	buf := &bytes.Buffer{}
	err = wire.WriteMessage(buf, msg)
	if err != nil {
		fmt.Fprint(w, err.Error())
		return
	}
	hub.msgQueue <- &Msg{from: clientFlag, message: buf.Bytes()}
	fmt.Fprint(w, "ok")
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
	go h.messageQueueHandler()
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

			isMaster := true
			for _, s := range h.serverPeers {
				if s.entity.ID < server.ID {
					isMaster = false
				}
			}
			if isMaster {
				h.serverCache.Clean()
			}
		}
	}
}

func isMasterServer(servers []database.Server, ID, exID uint64) bool {
	isMaster := true
	for _, s := range servers {
		if s.ID == exID {
			continue
		}
		if s.ID < ID { // 最小的 id 就是存活最久的服务，就是主服务
			isMaster = false
		}
	}
	return isMaster
}

// 与其它服务器节点建立长连接
func (h *Hub) outPeerHandler() error {
	log.Println("start outPeerhandler")
	if h.config.Server.Mode == config.ModeSingle {
		return nil
	}
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
		// ID  不一样，IP和端口一样
		if server.IP == serverSelf.IP && server.Port == serverSelf.Port {
			h.serverCache.DelServer(server.ID)
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
					buf, _ := wire.MakeKillMessage(0, p.entity.ID, p.ID)
					fmt.Printf("kill client:%v \n", p.Peer.ID)
					p.PushMessage(buf, nil)
				} else {
					client, _ := h.clientCache.GetClient(peer.entity.ID)
					// 如果节点已经登陆到其它服务器，就发送一个MsgKillClient踢去另外一台服务上的连接
					if client != nil {
						buf, _ := wire.MakeKillMessage(0, peer.entity.ID, peer.ID)
						if server, ok := h.serverPeers[client.ServerID]; ok {
							server.PushMessage(buf, nil)
						}
					}
				}
				h.clientPeers[peer.entity.ID] = peer
				h.clientCache.AddClient(peer.entity)
				peerRegistAck(peer)
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
				if p, ok := h.clientPeers[peer.entity.ID]; ok {
					if p.Peer.ID != peer.Peer.ID {
						continue
					}
					delete(h.clientPeers, peer.entity.ID)
					h.clientCache.DelClient(peer.entity.ID)
					peer.Close()
					log.Printf("client %v disconnected", peer.Peer.ID)
				}
			case *ServerPeer:
				peer := p.peer.(*ServerPeer)
				if _, ok := h.serverPeers[peer.entity.ID]; ok {
					delete(h.serverPeers, peer.entity.ID)
					delete(h.ServerSelf.OutServers, peer.entity.ID)
					peer.Close()
					log.Printf("server %v disconnected", peer.Peer.ID)
				}
			}
			if p.done != nil {
				p.done <- struct{}{}
			}
		}
	}
}

// login ack ,return a peerId to client
func peerRegistAck(peer *ClientPeer) {
	buf, _ := wire.MakeLoginAckMessage(0, peer.ID)
	peer.PushMessage(buf, nil)
}

// 处理消息queue
func (h *Hub) messageQueueHandler() {
	log.Println("start messageQueueHandler")
	pendingMsgs := list.New()

	// We keep the waiting flag so that we know if we have a pending message
	waiting := false

	// To avoid duplication below.
	queuePacket := func(msg *Msg, list *list.List, waiting bool) bool {
		if !waiting {
			h.msgRelay <- msg
		} else {
			list.PushBack(msg)
		}
		log.Println("panding message ", list.Len())
		// we are always waiting now.
		return true
	}
	for {
		select {
		case msg := <-h.msgQueue:
			h.messageLog.Write(msg.message)
			waiting = queuePacket(msg, pendingMsgs, waiting)
		case <-h.msgRelayDone:
			// log.Printf("message %v relayed \n", ID)
			next := pendingMsgs.Front()
			if next == nil {
				waiting = false
				continue
			}
			val := pendingMsgs.Remove(next)
			h.msgRelay <- val.(*Msg)
		}
	}
}

func (h *Hub) messageHandler() {
	log.Println("start messageHandler")
	for {
		select {
		case msg := <-h.msgRelay:
			header, err := h.messageRelay(msg)
			if err != nil {
				log.Println(err)
			}
			// log.Printf("message %v relaying \n", header.ID)
			h.msgRelayDone <- header.ID
		}
	}
}

var errMessageReceiverOffline = errors.New("Message Receiver is offline")

func (h *Hub) messageRelay(msg *Msg) (*wire.MessageHeader, error) {
	header, err := wire.ReadHeader(bytes.NewReader(msg.message))
	if err != nil {
		return nil, err
	}
	log.Println("messagehandler: a message to ", header.To)

	if header.Scope == wire.ScopeClient {
		to := header.To
		// 在当前服务器节点中找到了目标客户端
		if client, ok := h.clientPeers[to]; ok {
			client.PushMessage(msg.message, nil)
			return header, nil
		}

		// 读取目标client所在的服务器
		client, err := h.clientCache.GetClient(to)
		if err != nil {
			return header, err
		}

		if client == nil {
			return header, errMessageReceiverOffline
		}

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
		clients, err := h.groupCache.GetGroupMembers(group)
		if err != nil {
			return header, err
		}
		clen := len(clients)
		if clen == 0 {
			return header, nil
		}
		log.Println("group message to clients:", clen)
		for _, clientID := range clients {
			if client, ok := h.clientPeers[clientID]; ok {
				client.PushMessage(msg.message, nil)
			} else {
				// 如果发现用户不存在就清理掉
				h.groupCache.Leave(group, clientID)
			}
		}
	}
	return header, nil
}

func saveMessagesToDb(messageStore database.MessageStore, bufs []*bytes.Buffer) error {
	messages := make([]*database.ChatMsg, 0)
	for _, buf := range bufs {
		msg, err := wire.ReadMessage(buf)
		if err != nil {
			fmt.Println(err)
			continue
		}
		chatMsg := msg.(*wire.Msgchat)
		dbmsg := &database.ChatMsg{
			From:     chatMsg.From,
			To:       chatMsg.Header().To,
			Scope:    chatMsg.Header().Scope,
			Type:     chatMsg.Type,
			Text:     chatMsg.Text,
			Extra:    chatMsg.Extra,
			CreateAt: time.Now(),
		}
		messages = append(messages, dbmsg)
	}
	err := messageStore.Save(messages...)
	if err != nil {
		return err
	}
	log.Printf("save messages : %v ", len(messages))
	return nil
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
