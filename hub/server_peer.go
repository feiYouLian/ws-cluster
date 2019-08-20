package hub

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/peer"
	"github.com/ws-cluster/wire"
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
func (p *ServerPeer) OnMessage(message *wire.Message) error {

	if !message.Header.Dest.IsEmpty() {
		p.hub.msgQueue <- &Msg{from: serverFlag, message: message}
	}
	return nil
}

// OnDisconnect 对方断开接连
func (p *ServerPeer) OnDisconnect() error {
	// log.Printf("server %v disconnected ; from %v:%v", p.entity.ID, p.entity.IP, p.entity.Port)
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
	// log.Printf("connecting to %s", u.String())

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

	peer := peer.NewPeer(fmt.Sprintf("%v@%v:%v", server.ID, server.IP, server.Port),
		&peer.Config{
			Listeners: &peer.MessageListeners{
				OnMessage:    serverPeer.OnMessage,
				OnDisconnect: serverPeer.OnDisconnect,
			},
			PingPeriod:     time.Second * 20,
			PongWait:       time.Second * 60,
			MaxMessageSize: h.config.Peer.MaxMessageSize,
		})

	serverPeer.Peer = peer
	// 被动接受的连接
	serverPeer.SetConnection(conn)

	return serverPeer, nil
}
