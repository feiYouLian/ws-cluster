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

	"github.com/gorilla/websocket"

	// cmap "github.com/orcaman/concurrent-map"
	"github.com/ws-cluster/config"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/filelog"
	"github.com/ws-cluster/wire"
)

const (
	pingInterval = time.Second * 3

	useForAddClientPeer = uint8(1)
	useForDelClientPeer = uint8(2)
	useForAddServerPeer = uint8(3)
	useForDelServerPeer = uint8(4)
	useForRelayMessage  = uint8(5)
)

var (
	// ErrPeerNoFound peer is not in this server
	ErrPeerNoFound = errors.New("peer is not in this server")
)

// Resp Resp
type Resp struct {
	Status uint8
	Err    error
	Body   wire.Protocol
}

// Packet  Packet to hub
type Packet struct {
	from    wire.Addr //
	use     uint8
	content interface{}
	resp    chan *Resp
}

// Server 服务器对象
type Server struct {
	Addr   wire.Addr // logic address
	URL    *url.URL
	Secret string
}

// Hub 是一个服务中心，所有 clientPeer
type Hub struct {
	upgrader *websocket.Upgrader
	config   *config.Config
	Server   *Server // self
	// clientPeers 缓存客户端节点数据
	clientPeers map[wire.Addr]*ClientPeer
	// serverPeers 缓存服务端节点数据
	serverPeers map[wire.Addr]*ServerPeer
	groups      map[wire.Addr]*Group
	location    map[wire.Addr]wire.Addr // client location in server

	messageLog *filelog.FileLog

	packetQueue     chan *Packet
	packetRelay     chan *Packet
	packetRelayDone chan *Packet
	quit            chan struct{}
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
	serverAddr, _ := wire.NewServerAddr(uint32(cfg.Server.Domain), cfg.Server.ID)
	hub := &Hub{
		upgrader:        upgrader,
		config:          cfg,
		clientPeers:     make(map[wire.Addr]*ClientPeer, 10000),
		serverPeers:     make(map[wire.Addr]*ServerPeer, 10),
		location:        make(map[wire.Addr]wire.Addr, 10000),
		groups:          make(map[wire.Addr]*Group, 100),
		packetQueue:     make(chan *Packet, 1),
		packetRelay:     make(chan *Packet, 1),
		packetRelayDone: make(chan *Packet, 1),
		messageLog:      messageLog,
		quit:            make(chan struct{}),
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

	go h.packetHandler()
	go h.packetQueueHandler()

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

// 处理消息queue
func (h *Hub) packetQueueHandler() {
	log.Println("start packetQueueHandler")
	pendingMsgs := list.New()

	// We keep the waiting flag so that we know if we have a pending message
	waiting := false

	// To avoid duplication below.
	queuePacket := func(packet *Packet, list *list.List, waiting bool) bool {
		if !waiting {
			h.packetRelay <- packet
		} else {
			list.PushBack(packet)
		}
		// log.Println("panding message ", list.Len())
		// we are always waiting now.
		return true
	}
	for {
		select {
		case packet := <-h.packetQueue:
			if packet.resp == nil { //throw err
				packet.resp = make(chan *Resp)
				go func() {
					<-packet.resp
				}()
			}

			if packet.use == useForRelayMessage {
				message := packet.content.(*wire.Message)
				buf := &bytes.Buffer{}
				message.Encode(buf)
				err := h.messageLog.Write(buf.Bytes())
				if err != nil {
					packet.resp <- &Resp{
						Status: wire.MsgStatusException,
						Err:    err,
					}
					continue
				}
			}
			waiting = queuePacket(packet, pendingMsgs, waiting)
		case <-h.packetRelayDone:
			// log.Printf("message %v relayed \n", ID)
			next := pendingMsgs.Front()
			if next == nil {
				waiting = false
				continue
			}
			val := pendingMsgs.Remove(next)
			h.packetRelay <- val.(*Packet)
		}
	}
}

func (h *Hub) packetHandler() {
	log.Println("start packetHandler")
	for {
		select {
		case packet := <-h.packetRelay:
			switch packet.use {
			case useForAddClientPeer:
				h.handleClientPeerRegistPacket(packet.from, packet.content.(*ClientPeer), packet.resp)
			case useForDelClientPeer:
				h.handleClientPeerUnregistPacket(packet.from, packet.content.(*ClientPeer), packet.resp)
			case useForAddServerPeer:
				h.handleServerPeerRegistPacket(packet.from, packet.content.(*ServerPeer), packet.resp)
			case useForDelServerPeer:
				h.handleServerPeerUnregistPacket(packet.from, packet.content.(*ServerPeer), packet.resp)
			case useForRelayMessage:
				message := packet.content.(*wire.Message)
				header := message.Header
				h.recordSession(packet.from, header)
				if packet.from.Type() == wire.AddrServer && header.Source.Type() == wire.AddrPeer { //如果是转发过来的消息，就记录发送者的定位
					h.recordLocation(packet.from, message)
				}
				if header.Dest == h.Server.Addr { // if dest address is self
					h.handleLogicPacket(packet.from, message, packet.resp)
				} else {
					h.handleRelayPacket(packet.from, message, packet.resp)
				}
			}

			h.packetRelayDone <- packet
		}
	}
}

func (h *Hub) recordSession(from wire.Addr, header *wire.Header) {
	if header.Source.Type() == wire.AddrPeer {
		if peer, has := h.clientPeers[header.Dest]; has {
			if from.Type() == wire.AddrPeer { // source and dest peer are in same server
				peer.AddSession(header.Source, h.Server.Addr)
			} else {
				peer.AddSession(header.Source, from)
			}
		}
	}
}

// record visiting client peer location if this message is relaid by a server peer
func (h *Hub) recordLocation(from wire.Addr, message *wire.Message) {
	header := message.Header
	if _, has := h.location[header.Source]; has {
		return
	}
	dest := header.Dest
	h.location[header.Source] = from
	// A locating message is sent to the source server if dest is in this server, let it know the dest client is in this server.
	// so the server can directly send the same dest message to this server on next time
	if _, has := h.clientPeers[dest]; has {
		loc := wire.MakeEmptyHeaderMessage(wire.MsgTypeLoc, &wire.MsgLoc{
			Target: header.Source,
			Peer:   dest,
			In:     h.Server.Addr,
		})
		loc.Header.Dest = from
		loc.Header.Source = h.Server.Addr
		if speer, has := h.serverPeers[from]; has {
			speer.PushMessage(loc, nil)
		}
	}
}

func (h *Hub) handleClientPeerRegistPacket(from wire.Addr, peer *ClientPeer, resp chan<- *Resp) {
	packet := wire.MakeEmptyHeaderMessage(wire.MsgTypeKill, &wire.MsgKill{
		LoginAt: uint64(time.Now().UnixNano() / 1000000),
	})
	packet.Header.Source = peer.Addr
	packet.Header.Dest = peer.Addr // same addr

	if oldpeer, ok := h.clientPeers[peer.Addr]; ok {
		oldpeer.PushMessage(packet, nil)
	}
	h.broadcast(packet) // 广播此消息到其它服务器节点

	h.clientPeers[peer.Addr] = peer

	resp <- &Resp{
		Status: wire.MsgStatusOk,
	}
	return
}

func (h *Hub) handleClientPeerUnregistPacket(from wire.Addr, peer *ClientPeer, resp chan<- *Resp) {
	if alivePeer, ok := h.clientPeers[peer.Addr]; ok {
		if alivePeer.RemoteAddr != peer.RemoteAddr { // this two peer are different connection, ignore unregister
			resp <- &Resp{Status: wire.MsgStatusOk}
			return
		}
		delete(h.clientPeers, peer.Addr)

		// leave groups
		alivePeer.Groups.Each(func(elem interface{}) bool {
			gAddr := elem.(wire.Addr)
			if group, has := h.groups[gAddr]; has {
				group.packet <- &GroupPacket{useForLeave, peer}
			}
			return false
		})

		// notice other server your are offline
		for server, peers := range peer.getAllSessionServers() {
			offline := wire.MakeEmptyHeaderMessage(wire.MsgTypeOffline, &wire.MsgOffline{
				Peer:    peer.Addr,
				Targets: peers,
				Notice:  peer.OfflineNotice,
			})
			offline.Header.Source = h.Server.Addr

			if speer, has := h.serverPeers[server]; has {
				offline.Header.Dest = server
				speer.PushMessage(offline, nil)
			} else {
				offline.Header.Dest = h.Server.Addr // send to logic handler
				// the session is in local server
				h.packetQueue <- &Packet{
					from:    h.Server.Addr,
					use:     useForRelayMessage,
					content: offline,
				}
			}
		}

		// if peer.OfflineNotice { //notice all clients that you are offline
		// 	peer.Sessions.Each(func(elem interface{}) bool {
		// 		session := elem.(*Session)
		// 		offline := wire.MakeEmptyHeaderMessage(wire.MsgTypeOffline, &wire.MsgOffline{
		// 			Peer:    peer.Addr,
		// 			Targets: []wire.Addr{},
		// 		})
		// 		offline.Header.Source = h.Server.Addr
		// 		offline.Header.Dest = session.PeerAddr

		// 		if cpeer, has := h.clientPeers[session.ServerAddr]; has {
		// 			cpeer.PushMessage(offline, nil)
		// 		} else if speer, has := h.serverPeers[session.ServerAddr]; has {
		// 			speer.PushMessage(offline, nil)
		// 		}
		// 		return false
		// 	})
		// }

	}
	resp <- &Resp{Status: wire.MsgStatusOk}
	return
}

func (h *Hub) handleServerPeerRegistPacket(from wire.Addr, peer *ServerPeer, resp chan<- *Resp) {
	h.serverPeers[peer.Addr] = peer
	resp <- &Resp{Status: wire.MsgStatusOk}
	return
}

func (h *Hub) handleServerPeerUnregistPacket(from wire.Addr, peer *ServerPeer, resp chan<- *Resp) {
	delete(h.serverPeers, peer.Addr)
	resp <- &Resp{Status: wire.MsgStatusOk}
	return
}

func (h *Hub) handleRelayPacket(from wire.Addr, message *wire.Message, resp chan<- *Resp) {
	header := message.Header
	dest := header.Dest
	var response = Resp{
		Status: wire.MsgStatusOk,
	}
	defer func() {
		resp <- &response
	}()
	if dest.Type() == wire.AddrPeer {
		// 在当前服务器节点中找到了目标客户端
		if cpeer, ok := h.clientPeers[dest]; ok {
			cpeer.PushMessage(message, nil) //errchan pass to peer
			return
		}
		if from.Type() == wire.AddrServer { //dest no found in this server .then throw out message
			response.Err = ErrPeerNoFound
			return
		}
		// message sent from client directly
		serverAddr, has := h.location[dest]
		if !has { // 如果找不到定位，广播此消息
			h.broadcast(message)
		} else {
			if speer, ok := h.serverPeers[serverAddr]; ok {
				speer.PushMessage(message, nil)
			}
		}
		response.Err = ErrPeerNoFound
	} else {
		// 如果消息是直接来源于 client。就转发到其它服务器
		if from.Type() == wire.AddrPeer {
			h.broadcast(message)
		}

		if dest.Type() == wire.AddrGroup {
			// 消息异步发送到群中所有用户
			if group, has := h.groups[dest]; has {
				group.packet <- &GroupPacket{useForMessage, message}
			}
		} else if dest.Type() == wire.AddrBroadcast {
			// 消息异步发送到群中所有用户
			h.sendToDomain(dest, message)
		}
	}
}

func (h *Hub) handleLogicPacket(from wire.Addr, message *wire.Message, resp chan<- *Resp) {
	header := message.Header
	body := message.Body
	var response = Resp{
		Status: wire.MsgStatusOk,
	}
	defer func() {
		resp <- &response
	}()

	// if header.Source.Type() == wire.AddrPeer {
	// 	if _, has := h.clientPeers[header.Source]; !has {
	// 		resp := wire.MakeEmptyRespMessage(header, wire.MsgStatusSourceNoFound)
	// 		h.responseMessage(from, resp)
	// 		return
	// 	}
	// }

	switch header.Command {
	case wire.MsgTypeGroupInOut:
		msgGroup := body.(*wire.MsgGroupInOut)
		peer := h.clientPeers[header.Source]
		if peer == nil {
			response.Err = ErrPeerNoFound
			return
		}
		for _, group := range msgGroup.Groups {
			switch msgGroup.InOut {
			case wire.GroupIn:
				peer.Groups.Add(group) //record to peer
				if _, ok := h.groups[group]; !ok {
					h.groups[group] = NewGroup(group)
				}
				h.groups[group].packet <- &GroupPacket{useForJoin, peer}
			case wire.GroupOut:
				peer.Groups.Remove(group)
				if g, has := h.groups[group]; has {
					g.packet <- &GroupPacket{useForLeave, peer}

					if g.MemCount == 0 {
						if len(h.groups) > 1000 { // clean group
							g.Exit() //stop
							delete(h.groups, group)
						}
					}
				}

			}
		}
	case wire.MsgTypeLoc: //handle location message
		msgLoc := body.(*wire.MsgLoc)
		h.location[msgLoc.Peer] = msgLoc.In
		//  regist a server to peer whether it is successful
		peer := h.clientPeers[msgLoc.Target]
		peer.AddSession(msgLoc.Peer, msgLoc.In)
	case wire.MsgTypeOffline: //handle offline message
		msgOffline := body.(*wire.MsgOffline)
		delete(h.location, msgOffline.Peer)

		for _, target := range msgOffline.Targets {
			peer, has := h.clientPeers[target]
			if !has {
				continue
			}
			peer.DelSession(msgOffline.Peer)
			if msgOffline.Notice == 1 { //notice to client
				offlineNotice := wire.MakeEmptyHeaderMessage(wire.MsgTypeOfflineNotice, &wire.MsgOfflineNotice{
					Peer: msgOffline.Peer,
				})
				offlineNotice.Header.Dest = target
				peer.PushMessage(offlineNotice, nil)
			}
		}
	}
}

func (h *Hub) responseMessage(from wire.Addr, message *wire.Message) {
	if from.Type() == wire.AddrPeer {
		h.clientPeers[from].PushMessage(message, nil)
	} else if from.Type() == wire.AddrServer {
		h.serverPeers[from].PushMessage(message, nil)
	}
}

var errMessageReceiverOffline = errors.New("Message Receiver is offline")

// func (h *Hub) sendToGroup(group wire.Addr, message *wire.Message) {
// 	// 读取群用户列表。转发
// 	peers, has := h.groups[group]
// 	if !has {
// 		return
// 	}

// 	if peers.Cardinality() == 0 {
// 		return
// 	}

// 	peers.Each(func(elem interface{}) bool {
// 		elem.(*ClientPeer).PushMessage(message, nil)
// 		return false
// 	})

// }

func (h *Hub) sendToDomain(dest wire.Addr, message *wire.Message) {
	for addr, cpeer := range h.clientPeers {
		if addr.Domain() == dest.Domain() {
			cpeer.PushMessage(message, nil)
		}
	}
}

// broadcast message to all server
func (h *Hub) broadcast(message *wire.Message) {
	if len(h.serverPeers) == 0 {
		return
	}
	// errchan := make(chan error, h.serverPeers.Count())
	for _, speer := range h.serverPeers {
		speer.PushMessage(message, nil)
	}
}

func saveMessagesToDb(messageStore database.MessageStore, bufs []*bytes.Buffer) error {
	messages := make([]interface{}, 0)
	for _, buf := range bufs {
		packet := new(wire.Message)
		if err := packet.Decode(buf); err != nil {
			fmt.Println(err)
			continue
		}
		header := packet.Header
		if header.Command != wire.MsgTypeChat {
			continue
		}
		body := packet.Body.(*wire.Msgchat)
		if header.Dest.Type() == wire.AddrPeer {
			dbmsg := &database.ChatMsg{
				FromDomain: header.Source.Domain(),
				ToDomain:   header.Dest.Domain(),
				From:       header.Source.Address(),
				To:         header.Dest.Address(),
				Type:       body.Type,
				Text:       body.Text,
				Extra:      body.Extra,
				CreateAt:   time.Now(),
			}
			messages = append(messages, dbmsg)
		} else if header.Dest.Type() == wire.AddrGroup {
			dbmsg := &database.RoomMsg{
				FromDomain: header.Source.Domain(),
				ToDomain:   header.Dest.Domain(),
				From:       header.Source.Address(),
				To:         header.Dest.Address(),
				Type:       body.Type,
				Text:       body.Text,
				Extra:      body.Extra,
				CreateAt:   time.Now(),
			}
			messages = append(messages, dbmsg)
		}
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

	for _, speer := range h.serverPeers {
		speer.Close()
	}

	for _, cpeer := range h.clientPeers {
		cpeer.Close()
	}

	time.Sleep(time.Second)
}
