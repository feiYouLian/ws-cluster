package hub

import (
	"log"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/gorilla/websocket"
	"github.com/ws-cluster/peer"
	"github.com/ws-cluster/wire"
)

// Session Session
type Session struct {
	PeerAddr   wire.Addr
	ServerAddr wire.Addr
	LatestIn   time.Time //latest message from the peer
	LatestOut  time.Time
}

// ClientPeer 代表一个客户端节点，消息收发的处理逻辑
type ClientPeer struct {
	*peer.Peer
	LoginAt time.Time  //second
	Server  *Server    // the server you logined in
	Groups  mapset.Set //all your groups
	// all of clients communicated with you,
	// you must notice the servers by sending a offline message when you logout
	Sessions      map[wire.Addr]*Session
	OfflineNotice uint8
	packet        chan<- *Packet
}

// OnMessage 接收消息
func (p *ClientPeer) OnMessage(message *wire.Message) error {
	respchan := make(chan *Resp)

	if message.Header.Dest.IsEmpty() { // is command message
		message.Header.Dest = p.Server.Addr
	}
	p.packet <- &Packet{from: p.Addr, use: useForRelayMessage, content: message, resp: respchan}

	resp := <-respchan
	respMessage := wire.MakeEmptyRespMessage(message.Header, resp.Status)
	p.PushMessage(respMessage, nil)
	log.Println("message", message.Header.String(), "resp status:", respMessage.Header.Status)

	return nil
}

// OnDisconnect 接连断开
func (p *ClientPeer) OnDisconnect() error {
	respchan := make(chan *Resp)
	p.packet <- &Packet{from: p.Addr, use: useForDelClientPeer, content: p, resp: respchan}
	<-respchan
	log.Printf("client %v@%v disconnected", p.Addr.String(), p.RemoteAddr)
	return nil
}

func newClientPeer(addr wire.Addr, remoteAddr string, offlineNotice uint8, h *Hub, conn *websocket.Conn) (*ClientPeer, error) {
	clientPeer := &ClientPeer{
		packet:        h.packetQueue,
		Server:        h.Server,
		OfflineNotice: offlineNotice,
		Groups:        mapset.NewThreadUnsafeSet(),
		Sessions:      make(map[wire.Addr]*Session, 0),
	}
	peer := peer.NewPeer(addr, remoteAddr, &peer.Config{
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

// DelSession DelSession
func (p *ClientPeer) DelSession(peer wire.Addr) bool {
	if _, has := p.Sessions[peer]; has {
		return false
	}
	delete(p.Sessions, peer)
	return true
}

// AddSession AddSession
func (p *ClientPeer) AddSession(peer, server wire.Addr) bool {
	if _, has := p.Sessions[peer]; has {
		return false
	}
	p.Sessions[peer] = &Session{
		PeerAddr:   peer,
		ServerAddr: server,
	}
	return true
}

func (p *ClientPeer) getAllSessionServers() map[wire.Addr][]wire.Addr {
	servers := make(map[wire.Addr][]wire.Addr, 0)
	for _, session := range p.Sessions {
		_, has := servers[session.ServerAddr]
		if !has {
			servers[session.ServerAddr] = make([]wire.Addr, 0)
		}
		servers[session.ServerAddr] = append(servers[session.ServerAddr], session.PeerAddr)
	}
	return servers
}
