package main

import (
	"bytes"
	"io"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/wire"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// ClientPeer 代表一个客户端节点，消息收发的处理逻辑
type ClientPeer struct {
	conn   *websocket.Conn
	client *database.Client
	hub    *Hub
	send   chan wire.Message
}

// NewClientPeer create ClientPeer
func NewClientPeer(conn *websocket.Conn, client *database.Client) (*ClientPeer, error) {
	return &ClientPeer{
		conn:   conn,
		client: client,
	}, nil
}

func (p *ClientPeer) start() {
	go p.handleRead()
	go p.handleWrite()
}

func (p *ClientPeer) handleRead() {
	defer func() {
		p.hub.unregistClient <- p
		p.conn.Close()
	}()
	p.conn.SetReadLimit(maxMessageSize)
	p.conn.SetReadDeadline(time.Now().Add(pongWait))
	p.conn.SetPongHandler(func(string) error { p.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := p.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		p.hub.message <- message
	}
}

func (p *ClientPeer) handleWrite() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		p.conn.Close()
	}()
	for {
		select {
		case message, ok := <-p.send:
			p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				p.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := p.conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return
			}
			p.pushMessage(w, message)
			// Add queued chat messages to the current websocket message.
			n := len(p.send)
			for i := 0; i < n; i++ {
				message := <-p.send
				p.pushMessage(w, message)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (p *ClientPeer) pushMessage(w io.WriteCloser, message wire.Message) {
	if err := message.Encode(w); err == nil {
		if chatMsg, ok := message.(*wire.Msgchat); ok {
			if chatMsg.Type == wire.ChatTypeSingle {
				// 消息应答
				buf := &bytes.Buffer{}
				ack := wire.MsgchatAck{
					ID:    chatMsg.ID,
					To:    chatMsg.From,
					State: wire.MsgStateSent,
				}
				ack.Encode(buf)
				p.hub.message <- buf.Bytes()
			}
		}
	}
}
