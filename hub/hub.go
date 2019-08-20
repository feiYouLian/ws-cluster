package hub

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/gorilla/websocket"
	"github.com/ws-cluster/config"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/filelog"
	"github.com/ws-cluster/wire"
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

// Msg to hub
type Msg struct {
	from    byte
	message *wire.Message
	err     chan error
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
	// groupCache  database.GroupCache
	serverCache database.ServerCache

	// clientPeers 缓存客户端节点数据
	clientPeers map[wire.Addr]*ClientPeer
	// serverPeers 缓存服务端节点数据
	serverPeers map[uint64]*ServerPeer
	groups      map[wire.Addr]mapset.Set

	messageLog *filelog.FileLog
	loginLog   *filelog.FileLog

	register   chan *addPeer
	unregister chan *delPeer

	commandChan  chan *wire.Message
	msgQueue     chan *Msg
	msgRelay     chan *Msg
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
		upgrader:    upgrader,
		config:      cfg,
		ServerID:    cfg.Server.ID,
		clientCache: clientCache,
		serverCache: cfg.Cache.Server,
		// groupCache:   cfg.Cache.Group,
		clientPeers:  make(map[wire.Addr]*ClientPeer, 1000),
		serverPeers:  make(map[uint64]*ServerPeer, 10),
		groups:       make(map[wire.Addr]mapset.Set, 100),
		register:     make(chan *addPeer, 1),
		unregister:   make(chan *delPeer, 1),
		msgQueue:     make(chan *Msg, 1),
		msgRelay:     make(chan *Msg, 1),
		msgRelayDone: make(chan uint32, 1),
		commandChan:  make(chan *wire.Message, 1000),

		messageLog: messageLog,
		quit:       make(chan struct{}),
	}

	go httplisten(hub, &cfg.Server)

	return hub, nil
}

// Run start all handlers
func (h *Hub) Run() {

	go h.peerHandler()
	go h.messageHandler()
	go h.messageQueueHandler()
	go h.commandHandler()
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
				if oldpeer, ok := h.clientPeers[*peer.addr]; ok {
					// 如果节点已经登陆，就把前一个 peer踢下线
					msg, _ := wire.MakeEmptyHeaderMessage(wire.MsgTypeKill, &wire.MsgKill{
						PeerID: oldpeer.ID,
					})

					fmt.Printf("kill client:%v \n", oldpeer.ID)
					oldpeer.PushMessage(msg, nil)
				} else {
					client, _ := h.clientCache.GetClient(peer.entity.ID)
					// 如果节点已经登陆到其它服务器，就发送一个MsgKillClient踢去另外一台服务上的连接
					if client != nil {
						msg, _ := wire.MakeEmptyHeaderMessage(wire.MsgTypeKill, &wire.MsgKill{
							PeerID: client.PeerID,
						})
						msg.Header.Dest = peer.addr // same addr

						if server, ok := h.serverPeers[client.ServerID]; ok {
							server.PushMessage(msg, nil)
						}
					}
				}
				h.clientPeers[*peer.addr] = peer
				h.clientCache.AddClient(peer.entity)

				log.Printf("client %v connected", peer.ID)
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
				if p, ok := h.clientPeers[*peer.addr]; ok {
					if p.Peer.ID != peer.Peer.ID {
						continue
					}
					delete(h.clientPeers, *peer.addr)
					h.clientCache.DelClient(peer.entity.ID)
					log.Printf("client %v disconnected", peer.ID)
				}
			case *ServerPeer:
				peer := p.peer.(*ServerPeer)
				if _, ok := h.serverPeers[peer.entity.ID]; ok {
					delete(h.serverPeers, peer.entity.ID)
					delete(h.ServerSelf.OutServers, peer.entity.ID)
					log.Printf("server %v disconnected", peer.ID)
				}
			}
			if p.done != nil {
				p.done <- struct{}{}
			}
		}
	}
}

// login ack ,return a peerId to client
// func peerRegistAck(peer *ClientPeer) {
// 	buf, _ := wire.MakeLoginAckMessage(0, peer.ID)
// 	peer.PushMessage(buf, nil)
// }

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
		// log.Println("panding message ", list.Len())
		// we are always waiting now.
		return true
	}
	for {
		select {
		case msg := <-h.msgQueue:
			buf := &bytes.Buffer{}
			msg.message.Encode(buf)
			err := h.messageLog.Write(buf.Bytes())
			if msg.err != nil {
				msg.err <- err // error or nil
			}
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
			header := msg.message.Header
			dest := header.Dest
			// log.Println("messagehandler: a message to ", header.To)
			if dest.Type() == wire.AddrPeer {

				// 在当前服务器节点中找到了目标客户端
				if client, ok := h.clientPeers[*dest]; ok {
					client.PushMessage(msg.message, nil)
					continue
				}

				// 读取目标client所在的服务器
				client, err := h.clientCache.GetClient(dest.String())
				if err != nil || client == nil {
					continue
				}

				if server, ok := h.serverPeers[client.ServerID]; ok {
					server.PushMessage(msg.message, nil)
				}
			} else if dest.Type() == wire.AddrGroup {
				// 如果消息是直接来源于 client。就转发到其它服务器
				if msg.from == clientFlag {
					for _, serverpeer := range h.serverPeers {
						serverpeer.PushMessage(msg.message, nil)
					}
				}
				// 消息异步发送到群中所有用户
				go h.sendToGroup(dest, msg.message)
			}

			// log.Printf("message %v relaying \n", header.ID)
			h.msgRelayDone <- header.Seq
		}
	}
}

func (h *Hub) commandHandler() {
	log.Println("start commandHandler")
	for {
		select {
		case message := <-h.commandChan:
			header := message.Header

			// 处理消息逻辑
			switch header.Command {
			case wire.MsgTypeGroupInOut:
				msgGroup := message.Body.(*wire.MsgGroupInOut)
				for _, group := range msgGroup.Groups {
					if _, ok := h.groups[group]; !ok {
						h.groups[group] = mapset.NewSet()
					}

					switch msgGroup.InOut {
					case wire.GroupIn:
						h.groups[group].Add(*header.Source) // not pointer
					case wire.GroupOut:
						h.groups[group].Remove(*header.Source)

						if h.groups[group].Cardinality() == 0 {
							delete(h.groups, group)
						}
					}
				}
			}
		}
	}
}

var errMessageReceiverOffline = errors.New("Message Receiver is offline")

func (h *Hub) sendToGroup(group *wire.Addr, message *wire.Message) {
	// 读取群用户列表。转发
	addrs := h.groups[*group]

	if addrs.Cardinality() == 0 {
		return
	}
	if addrs.Cardinality() < 30 {
		log.Println("group message to clients:", addrs.ToSlice())
	}

	for elem := range addrs.Iterator().C {
		addr := elem.(wire.Addr)
		if client, ok := h.clientPeers[addr]; ok {
			client.PushMessage(message, nil)
		} else {
			// 如果发现用户不存在就清理掉
			msg, _ := wire.MakeEmptyHeaderMessage(wire.MsgTypeGroupInOut, &wire.MsgGroupInOut{
				InOut:  wire.GroupOut,
				Groups: []wire.Addr{*group},
			})
			msg.Header.Source = &addr
			h.commandChan <- msg
		}
	}
}

func saveMessagesToDb(messageStore database.MessageStore, bufs []*bytes.Buffer) error {
	messages := make([]*database.ChatMsg, 0)
	for _, buf := range bufs {
		msg := new(wire.Message)
		if err := msg.Encode(buf); err != nil {
			fmt.Println(err)
			continue
		}
		header := msg.Header
		body := msg.Body.(*wire.Msgchat)
		dbmsg := &database.ChatMsg{
			FromDomain: header.Source.Domain(),
			ToDomain:   header.Dest.Domain(),
			From:       header.Source.Address(),
			To:         header.Dest.Address(),
			Scope:      header.Dest.Type(),
			Type:       body.Type,
			Text:       body.Text,
			Extra:      body.Extra,
			CreateAt:   time.Now(),
		}
		messages = append(messages, dbmsg)
	}
	err := messageStore.Save(messages...)
	if err != nil {
		return err
	}
	// log.Printf("save messages : %v ", len(messages))
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
