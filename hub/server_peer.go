package hub

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ws-cluster/peer"
	"github.com/ws-cluster/wire"
)

// ServerPeer 代表一个服务器节点，每个服务器节点都会建立与其它服务器节点的接连，
// 这个对象用于处理跨服务节点消息收发。
type ServerPeer struct {
	*peer.Peer
	URL        *url.URL // connect url
	IsOut      bool     // be connected server
	HostServer *Server  // host

	msgchan   chan<- *Packet
	closechan chan<- *delPeer
}

// OnMessage 接收消息
func (p *ServerPeer) OnMessage(message *wire.Message) error {
	err := make(chan error)
	p.msgchan <- &Packet{from: fromServer, fromID: p.ID, message: message, err: err}

	// header := message.Header
	// if !header.Dest.IsEmpty() {
	// 	log.Printf("message %v to %v , Type: %v", header.Source.String(), header.Dest.String(), header.Command)
	// 	// errchan := make(chan error)
	// 	// 消息转发
	// } else {
	// 	p.hub.commandChan <- message
	// }

	return <-err
}

// OnDisconnect 对方断开接连
func (p *ServerPeer) OnDisconnect() error {
	// log.Printf("server %v disconnected ; from %v:%v", p.entity.ID, p.entity.IP, p.entity.Port)

	done := make(chan struct{})
	p.closechan <- &delPeer{peer: p, done: done}
	<-done
	log.Printf("client %v disconnected", p.ID)

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
	io.WriteString(h, fmt.Sprintf("%v%v", host.Addr.String(), host.URL.String()))
	io.WriteString(h, host.Secret)
	digest := hex.EncodeToString(h.Sum(nil))

	header := http.Header{}
	header.Add("id", host.Addr.String())
	header.Add("url", host.URL.String())
	header.Add("digest", digest)

	// log.Printf("connecting to %s", u.String())

	dialar := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 3 * time.Second,
	}

	conn, _, err := dialar.Dial(p.URL.String(), header)
	if err != nil {
		log.Println("dial:", err)
		return err
	}
	p.SetConnection(conn)
	return nil
}

// newServerPeer 主动去连接另一台服务节点器
func newServerPeer(h *Hub, server *Server) (*ServerPeer, error) {

	serverPeer := &ServerPeer{
		HostServer: h.Server,
		URL:        server.URL,
		IsOut:      true,
		msgchan:    h.msgQueue,
		closechan:  h.unregister,
	}

	peer := peer.NewPeer(server.Addr.String(), "",
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
func bindServerPeer(h *Hub, conn *websocket.Conn, server *Server, remoteAddr string) (*ServerPeer, error) {
	serverPeer := &ServerPeer{
		HostServer: h.Server,
		URL:        server.URL,
		IsOut:      false,
		msgchan:    h.msgQueue,
		closechan:  h.unregister,
	}

	peer := peer.NewPeer(server.Addr.String(), remoteAddr,
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
