package hub

import (
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/ws-cluster/config"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/filelog"
	"github.com/ws-cluster/wire"
)

const (
	pingInterval = time.Second * 3
)

const (
	fromClient = 1
	fromServer = 3
)

// Packet  Packet to hub
type Packet struct {
	from    uint8 // fromClient or fromServer
	fromID  string
	message *wire.Message
	err     chan error
}

// Server 服务器对象
type Server struct {
	Addr   wire.Addr // logic address
	URL    *url.URL
	Secret string
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
	upgrader *websocket.Upgrader
	config   *config.Config
	Server   *Server // self
	// clientCache database.ClientCache
	// groupCache  database.GroupCache
	// serverCache database.ServerCache

	// clientPeers 缓存客户端节点数据
	clientPeers cmap.ConcurrentMap
	// serverPeers 缓存服务端节点数据
	serverPeers cmap.ConcurrentMap
	groups      map[wire.Addr]mapset.Set
	location    cmap.ConcurrentMap // client location in server

	messageLog *filelog.FileLog

	register   chan *addPeer
	unregister chan *delPeer

	commandChan  chan *Packet
	msgQueue     chan *Packet
	msgRelay     chan *Packet
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
	serverAddr, _ := wire.NewAddr(wire.AddrServer, uint32(cfg.Server.Domain), cfg.Server.ID)
	hub := &Hub{
		upgrader:     upgrader,
		config:       cfg,
		clientPeers:  cmap.New(),
		serverPeers:  cmap.New(),
		location:     cmap.New(),
		groups:       make(map[wire.Addr]mapset.Set, 100),
		register:     make(chan *addPeer, 1),
		unregister:   make(chan *delPeer, 1),
		msgQueue:     make(chan *Packet, 1),
		msgRelay:     make(chan *Packet, 1),
		msgRelayDone: make(chan uint32, 1),
		commandChan:  make(chan *Packet, 1000),
		messageLog:   messageLog,
		quit:         make(chan struct{}),
		Server: &Server{
			Addr:   *serverAddr,
			Secret: cfg.Server.Secret,
			URL:    &url.URL{Scheme: "ws", Host: fmt.Sprintf("%s:%d", wire.GetOutboundIP().String(), cfg.Server.ListenPort), Path: "/server"},
		},
	}

	go httplisten(hub, &cfg.Server)

	log.Printf("server[%v] start up", serverAddr.String())

	return hub, nil
}

// Run start all handlers
func (h *Hub) Run() {

	go h.peerHandler()
	go h.messageHandler()
	go h.messageQueueHandler()
	go h.commandHandler()

	// 连接到其它服务器节点,并且对放开放服务
	h.outPeerHandler()

	<-h.quit
}

// func (h *Hub) pingHandler() error {
// 	if h.config.Server.Mode == config.ModeSingle {
// 		return nil
// 	}
// 	ticker := time.NewTicker(time.Second * 3)
// 	defer ticker.Stop()
// 	for {
// 		select {
// 		case <-ticker.C:
// 			server := h.ServerSelf
// 			server.ClientNum = h.clientPeers.Count()
// 			h.serverCache.SetServer(server)

// 			isMaster := true
// 			for _, s := range h.serverPeers {
// 				if s.entity.ID < server.ID {
// 					isMaster = false
// 				}
// 			}
// 			if isMaster {
// 				h.serverCache.Clean()
// 			}
// 		}
// 	}
// }

// 与其它服务器节点建立长连接
func (h *Hub) outPeerHandler() error {
	log.Println("start outPeerhandler")
	if h.config.Server.Mode == config.ModeSingle {
		return nil
	}
	// servers, err := h.serverCache.GetServers()

	// serverSelf := database.Server{
	// 	ID:         h.config.Server.ID,
	// 	IP:         wire.GetOutboundIP().String(),
	// 	Port:       h.config.Server.Listen,
	// 	StartAt:    time.Now().Unix(),
	// 	ClientNum:  0,
	// 	OutServers: make(map[string]string),
	// }
	// h.ServerSelf = &serverSelf

	// // 主动连接到其它节点
	// for _, server := range servers {
	// 	if server.ID == serverSelf.ID {
	// 		continue
	// 	}
	// 	// ID  不一样，IP和端口一样
	// 	if server.IP == serverSelf.IP && server.Port == serverSelf.Port {
	// 		h.serverCache.DelServer(server.ID)
	// 		continue
	// 	}
	// 	serverPeer, err := newServerPeer(h, &server)
	// 	if err != nil {
	// 		continue
	// 	}
	// 	h.register <- &addPeer{peer: serverPeer, done: nil}
	// }

	// // 记录到远程缓存中
	// h.serverCache.SetServer(&serverSelf)
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
				if elem, ok := h.clientPeers.Get(peer.ID); ok {
					oldpeer := elem.(*ClientPeer)
					// 如果节点已经登陆，就把前一个 peer踢下线
					msg := wire.MakeEmptyHeaderMessage(wire.MsgTypeKill, &wire.MsgKill{
						PeerID: oldpeer.ID,
					})

					fmt.Printf("kill client:%v \n", oldpeer.ID)
					oldpeer.PushMessage(msg, nil)
				}
				msg := wire.MakeEmptyHeaderMessage(wire.MsgTypeKill, &wire.MsgKill{})
				msg.Header.Dest = peer.Addr // same addr
				h.broadcast(msg)            // 广播此消息到其它服务器节点

				h.clientPeers.Set(peer.ID, peer)
			case *ServerPeer:
				peer := p.peer.(*ServerPeer)
				h.serverPeers.Set(peer.ID, peer)
			}
			if p.done != nil {
				p.done <- struct{}{}
			}
		case p := <-h.unregister:
			switch p.peer.(type) {
			case *ClientPeer:
				peer := p.peer.(*ClientPeer)
				if elem, ok := h.clientPeers.Get(peer.ID); ok {
					alivePeer := elem.(*ClientPeer)
					if alivePeer.RemoteAddr != peer.RemoteAddr { // this two peer are different connection, ignore unregister
						continue
					}
					h.clientPeers.Remove(peer.ID)
				}
			case *ServerPeer:
				peer := p.peer.(*ServerPeer)
				if h.serverPeers.Has(peer.ID) {
					h.serverPeers.Remove(peer.ID)
				}
			}
			if p.done != nil {
				p.done <- struct{}{}
			}
		}
	}
}

// 处理消息queue
func (h *Hub) messageQueueHandler() {
	log.Println("start messageQueueHandler")
	pendingMsgs := list.New()

	// We keep the waiting flag so that we know if we have a pending message
	waiting := false

	// To avoid duplication below.
	queuePacket := func(msg *Packet, list *list.List, waiting bool) bool {
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
			header := msg.message.Header
			if header.Dest.IsEmpty() || header.Dest == h.Server.Addr {
				h.commandChan <- msg
				continue
			}
			buf := &bytes.Buffer{}
			err := msg.message.Encode(buf)
			if err != nil {
				msg.err <- err
				continue
			}
			err = h.messageLog.Write(buf.Bytes())
			if err != nil {
				msg.err <- err
				continue
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
			h.msgRelay <- val.(*Packet)
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
			if msg.from == fromServer { //如果是转发过来的消息，就记录发送者的定位
				source := header.Source.String()
				if !h.location.Has(source) {
					h.location.Set(header.Source.String(), msg.fromID)

					// A locating message is sent to the source server , let it know the dest client is in this server.
					// so the server can directly send the same dest message to this server on next time
					if h.clientPeers.Has(dest.String()) {
						loc := wire.MakeEmptyHeaderMessage(wire.MsgTypeLoc, &wire.MsgLoc{
							Peer: header.Dest,
							In:   h.Server.Addr,
						})
						ele, _ := h.serverPeers.Get(msg.fromID)
						ele.(*ServerPeer).PushMessage(loc, nil)
					}
				}
			}
			if dest.Type() == wire.AddrPeer {
				// 在当前服务器节点中找到了目标客户端
				if client, ok := h.clientPeers.Get(dest.String()); ok {
					client.(*ClientPeer).PushMessage(msg.message, nil)
					continue
				}
				if msg.from == fromServer { //dest no found in this server .then throw out message
					continue
				}
				// message sent from client directly
				serverID, has := h.location.Get(dest.String())
				if !has { // 如果找不到定位，广播此消息
					h.broadcast(msg.message)
				} else {
					if server, ok := h.serverPeers.Get(serverID.(string)); ok {
						server.(*ServerPeer).PushMessage(msg.message, nil)
					}
				}
			} else {
				// 如果消息是直接来源于 client。就转发到其它服务器
				if msg.from == fromClient {
					h.broadcast(msg.message)
				}

				if dest.Type() == wire.AddrGroup {
					// 消息异步发送到群中所有用户
					go h.sendToGroup(dest, msg.message)
				} else if dest.Type() == wire.AddrBroadcast {
					// 消息异步发送到群中所有用户
					go h.sendToDomain(dest, msg.message)
				}
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
		case msg := <-h.commandChan:
			header := msg.message.Header

			// 处理消息逻辑
			switch header.Command {
			case wire.MsgTypeGroupInOut:
				msgGroup := msg.message.Body.(*wire.MsgGroupInOut)
				for _, group := range msgGroup.Groups {
					if _, ok := h.groups[group]; !ok {
						h.groups[group] = mapset.NewSet()
					}

					switch msgGroup.InOut {
					case wire.GroupIn:
						h.groups[group].Add(header.Source) // not pointer
					case wire.GroupOut:
						h.groups[group].Remove(header.Source)

						if h.groups[group].Cardinality() == 0 {
							delete(h.groups, group)
						}
					}
				}
			case wire.MsgTypeLoc: //收到定位消息
				msgLoc := msg.message.Body.(*wire.MsgLoc)
				h.location.Set(msgLoc.Peer.String(), msgLoc.In.String)
			case wire.MsgTypeOffline: //收到节点离线消息
				msgOffline := msg.message.Body.(*wire.MsgOffline)
				h.location.Remove(msgOffline.Peer.String())
			}
		}
	}
}

var errMessageReceiverOffline = errors.New("Message Receiver is offline")

func (h *Hub) sendToGroup(group wire.Addr, message *wire.Message) {
	// 读取群用户列表。转发
	addrs := h.groups[group]

	if addrs.Cardinality() == 0 {
		return
	}
	if addrs.Cardinality() < 30 {
		log.Println("group message to clients:", addrs.ToSlice())
	}

	for elem := range addrs.Iterator().C {
		addr := elem.(wire.Addr)
		if client, ok := h.clientPeers.Get(addr.String()); ok {
			client.(*ClientPeer).PushMessage(message, nil)
		} else {
			// 如果发现用户不存在就清理掉
			msg := wire.MakeEmptyHeaderMessage(wire.MsgTypeGroupInOut, &wire.MsgGroupInOut{
				InOut:  wire.GroupOut,
				Groups: []wire.Addr{group},
			})
			msg.Header.Source = addr
			h.commandChan <- &Packet{message: msg}
		}
	}
}

func (h *Hub) sendToDomain(dest wire.Addr, message *wire.Message) {
	for elem := range h.clientPeers.Iter() {
		addr, _ := wire.NewPeerAddr(elem.Key)
		if addr.Domain() == dest.Domain() {
			elem.Val.(*ClientPeer).PushMessage(message, nil)
		}
	}
}

// broadcast message to all server
func (h *Hub) broadcast(message *wire.Message) {
	if h.serverPeers.Count() == 0 {
		return
	}
	// errchan := make(chan error, h.serverPeers.Count())
	for elem := range h.serverPeers.Iter() {
		elem.Val.(*ServerPeer).PushMessage(message, nil)
	}
}

func saveMessagesToDb(messageStore database.MessageStore, bufs []*bytes.Buffer) error {
	messages := make([]*database.ChatMsg, 0)
	for _, buf := range bufs {
		msg := new(wire.Message)
		if err := msg.Decode(buf); err != nil {
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
	// if h.config.Server.Mode == config.ModeCluster {
	// 	h.serverCache.DelServer(h.ServerID)
	// 	log.Println("clean server in cache")
	// }

	for elem := range h.clientPeers.Iter() {
		peer := elem.Val.(*ClientPeer)
		peer.Close()
	}

	for elem := range h.serverPeers.Iter() {
		peer := elem.Val.(*ServerPeer)
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
