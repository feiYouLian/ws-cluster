package hub

import (
	"log"

	"github.com/gorilla/websocket"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/peer"
	"github.com/ws-cluster/wire"
)

// ClientPeer 代表一个客户端节点，消息收发的处理逻辑
type ClientPeer struct {
	*peer.Peer
	addr   *wire.Addr
	hub    *Hub
	entity *database.Client
}

// OnMessage 接收消息
func (p *ClientPeer) OnMessage(message *wire.Message) error {
	header := message.Header

	if !header.Dest.IsEmpty() {
		log.Println("message forward to hub", header)
		// errchan := make(chan error)
		// 消息转发
		p.hub.msgQueue <- &Msg{from: clientFlag, message: message, err: nil}
	} else {
		p.hub.commandChan <- message
	}

	return nil
}

// OnDisconnect 接连断开
func (p *ClientPeer) OnDisconnect() error {
	p.hub.unregister <- &delPeer{peer: p, done: nil}
	return nil
}

func newClientPeer(addr *wire.Addr, h *Hub, conn *websocket.Conn, client *database.Client) (*ClientPeer, error) {
	clientPeer := &ClientPeer{
		hub:    h,
		entity: client,
		addr:   addr,
	}

	peer := peer.NewPeer(client.PeerID, &peer.Config{
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
