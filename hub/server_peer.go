package hub

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ws-cluster/peer"
	"github.com/ws-cluster/wire"
)

// ServerPeer 代表一个服务器节点，每个服务器节点都会建立与其它服务器节点的接连，
// 这个对象用于处理跨服务节点消息收发。
type ServerPeer struct {
	*peer.Peer
	IsOut      bool    // be connected server
	HostServer *Server // host
	Server     *Server // Server

	packet chan<- *Packet
}

// OnMessage 接收消息
func (p *ServerPeer) OnMessage(message *wire.Message) error {
	respchan := make(chan *Resp)
	p.packet <- &Packet{from: p.Addr, use: useForRelayMessage, content: message, resp: respchan}
	<-respchan
	// header := message.Header
	// if !header.Dest.IsEmpty() {
	// 	log.Printf("message %v to %v , Type: %v", header.Source.String(), header.Dest.String(), header.Command)
	// 	// errchan := make(chan error)
	// 	// 消息转发
	// } else {
	// 	p.hub.commandChan <- message
	// }

	return nil
}

// OnDisconnect 对方断开接连
func (p *ServerPeer) OnDisconnect() error {
	// log.Printf("server %v disconnected ; from %v:%v", p.entity.ID, p.entity.IP, p.entity.Port)

	respchan := make(chan *Resp)
	p.packet <- &Packet{from: p.Addr, use: useForDelServerPeer, content: p, resp: respchan}
	<-respchan

	log.Printf("server %v disconnected", p.Addr.String())

	// // 尝试重连
	// for p.reconnectTimes < reconnectTimes {
	// 	server, _ := p.hub.serverCache.GetServer(p.entity.ID)
	// 	// 如果服务器列表中不存在，说明服务宕机了
	// 	if server == nil {
	// 		p.hub.unregister <- &delPeer{peer: p}
	// 		return nil
	// 	}
	// 	p.reconnectTimes++
	// 	if err := p.connect(); err == nil {
	// 		// 重连成功
	// 		return nil
	// 	}

	// 	time.Sleep(time.Second * 3)
	// }

	// p.hub.unregister <- &delPeer{peer: p}
	return nil
}

func (p *ServerPeer) connect() error {
	host := p.HostServer
	// 生成加密摘要
	h := md5.New()
	io.WriteString(h, host.Addr.String())
	io.WriteString(h, host.Token)
	digest := hex.EncodeToString(h.Sum(nil))

	header := http.Header{}
	header.Add("id", host.Addr.String())
	header.Add("server_url", host.AdvertiseServerURL.String())
	header.Add("peer_url", host.AdvertiseClientURL.String())
	header.Add("digest", digest)

	// log.Printf("connecting to %s", u.String())

	dialar := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 3 * time.Second,
	}

	conn, resp, err := dialar.Dial(p.Server.AdvertiseServerURL.String(), header)
	if err != nil {
		log.Println("dial:", err)
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New("invaild peer")
	}
	p.SetConnection(conn)
	return nil
}

// newServerPeer 主动去连接另一台服务节点器
func newServerPeer(h *Hub, server *Server) (*ServerPeer, error) {
	serverPeer := &ServerPeer{
		// Addr:       server.Addr,
		HostServer: h.Server,
		Server:     server,
		IsOut:      true,
		packet:     h.packetQueue,
	}

	peer := peer.NewPeer(server.Addr, "",
		&peer.Config{
			Listeners: &peer.MessageListeners{
				OnMessage:    serverPeer.OnMessage,
				OnDisconnect: serverPeer.OnDisconnect,
			},
			PingPeriod:     time.Second * 20,
			PongWait:       time.Second * 30,
			MaxMessageSize: 1024 * 10, //1M
		})

	serverPeer.Peer = peer
	// 主动进行连接
	if err := serverPeer.connect(); err != nil {
		return nil, err
	}

	return serverPeer, nil
}

// bindServerPeer 处理其它服务器节点过来的连接
func bindServerPeer(h *Hub, conn *websocket.Conn, server *Server, remoteAddr string) (*ServerPeer, error) {
	serverPeer := &ServerPeer{
		HostServer: h.Server,
		Server:     server,
		IsOut:      false,
		packet:     h.packetQueue,
	}

	peer := peer.NewPeer(server.Addr, remoteAddr,
		&peer.Config{
			Listeners: &peer.MessageListeners{
				OnMessage:    serverPeer.OnMessage,
				OnDisconnect: serverPeer.OnDisconnect,
			},
			PingPeriod:     time.Second * 20,
			PongWait:       time.Second * 30,
			MaxMessageSize: 1024 * 10, //1M
		})

	serverPeer.Peer = peer
	// 被动接受的连接
	serverPeer.SetConnection(conn)

	return serverPeer, nil
}
