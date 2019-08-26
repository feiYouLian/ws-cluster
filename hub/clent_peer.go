package hub

import (
	"log"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/gorilla/websocket"
	"github.com/ws-cluster/peer"
	"github.com/ws-cluster/wire"
)

// ClientPeer 代表一个客户端节点，消息收发的处理逻辑
type ClientPeer struct {
	*peer.Peer
	LoginAt time.Time  //second
	Server  *Server    // the server you logined in
	Groups  mapset.Set //all your groups
	//record all server peer which sent message to you
	// you must notice them by sending a offline message when you logout
	FriServers mapset.Set

	packet chan<- *Packet
}

// OnMessage 接收消息
func (p *ClientPeer) OnMessage(message *wire.Message) error {
	errchan := make(chan error)
	// log.Println("receive msg", message.Header.String())
	if message.Header.Dest.IsEmpty() { // is command message
		message.Header.Dest = p.Server.Addr
	}
	p.packet <- &Packet{from: p.Addr, use: useForRelayMessage, content: message, err: errchan}

	err := <-errchan
	if err == nil {
		return nil
	}
	if err == ErrPeerNoFound {
		return err
	}
	header := message.Header
	if header.Command == wire.MsgTypeChat {
		ackmessage := wire.MakeEmptyHeaderMessage(wire.MsgTypeChatResp, &wire.MsgChatResp{
			Err: err.Error(),
		})
		ackmessage.Header.Source = p.Server.Addr
		ackmessage.Header.Dest = header.Source
		ackmessage.Header.AckSeq = message.Header.Seq
		p.PushMessage(ackmessage, nil)
	}

	return nil
}

// OnDisconnect 接连断开
func (p *ClientPeer) OnDisconnect() error {
	errchan := make(chan error)
	p.packet <- &Packet{from: p.Addr, use: useForDelClientPeer, content: p, err: errchan}
	<-errchan
	log.Printf("client %v@%v disconnected", p.Addr.String(), p.RemoteAddr)
	return nil
}

func newClientPeer(addr wire.Addr, remoteAddr string, h *Hub, conn *websocket.Conn) (*ClientPeer, error) {
	clientPeer := &ClientPeer{
		packet:     h.packetQueue,
		Server:     h.Server,
		Groups:     mapset.NewThreadUnsafeSet(),
		FriServers: mapset.NewThreadUnsafeSet(),
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
