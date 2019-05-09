package peer

import (
	"bytes"
	"container/list"
	"log"
	"sync/atomic"
	"time"

	"github.com/ws-cluster/wire"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	defaultWriteWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	defaultPongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	defaultPingPeriod = 30 * time.Second

	// Maximum message size allowed from peer.
	defaultMaxMessageSize = 512
)

// MessageListeners 消息监听
type MessageListeners struct {
	// OnGetAddr is invoked when a peer receives a getaddr bitcoin message.
	OnMessage func(msg []byte) error

	OnDisconnect func() error
}

// Config 节点配置
type Config struct {

	// Time allowed to write a message to the peer.
	WriteWait time.Duration
	// Time allowed to read the next pong message from the peer.
	PongWait time.Duration
	// Send pings to peer with this period. Must be less than pongWait.
	PingPeriod time.Duration
	// Maximum message size allowed from peer.
	MaxMessageSize int

	Listeners *MessageListeners
}

type outMessage struct {
	message []byte
	done    chan<- struct{}
}

// Peer 节点封装了 websocket 通信底层接口
type Peer struct {
	ID            string
	config        *Config
	conn          *websocket.Conn
	outQueue      chan outMessage
	sendQueue     chan outMessage
	sendDone      chan struct{}
	quit          chan struct{}
	queueQuit     chan struct{}
	timeConnected time.Time

	connected int32
}

// NewPeer 创建一个新的节点
func NewPeer(id string, config *Config) *Peer {
	if config.WriteWait == 0 {
		config.WriteWait = defaultWriteWait
	}
	if config.PongWait == 0 {
		config.PongWait = defaultPongWait
	}
	if config.PingPeriod == 0 {
		config.PingPeriod = defaultPingPeriod
	}
	if config.MaxMessageSize == 0 {
		config.MaxMessageSize = defaultMaxMessageSize
	}

	if config.PingPeriod >= config.PongWait {
		config.PingPeriod = (config.PongWait * 9) / 10
	}
	return &Peer{
		ID:        id,
		config:    config,
		outQueue:  make(chan outMessage, 1),
		sendQueue: make(chan outMessage, 1),
		sendDone:  make(chan struct{}, 1),
		quit:      make(chan struct{}),
		queueQuit: make(chan struct{}),
	}
}

// SetConnection bind connection , start
func (p *Peer) SetConnection(conn *websocket.Conn) {
	// Already connected?
	if !atomic.CompareAndSwapInt32(&p.connected, 0, 1) {
		return
	}

	p.conn = conn
	p.timeConnected = time.Now()

	p.start()
}

func (p *Peer) start() {
	go p.inMessageHandler()
	go p.outQueueHandler()
	go p.outMessageHandler()
	log.Printf("peer %v started", p.ID)
}

func (p *Peer) inMessageHandler() {
	defer func() {
		p.Close()
		p.config.Listeners.OnDisconnect()
	}()
	p.conn.SetReadLimit(int64(p.config.MaxMessageSize))
	p.conn.SetReadDeadline(time.Now().Add(p.config.PongWait))
	p.conn.SetPongHandler(func(string) error {
		p.conn.SetReadDeadline(time.Now().Add(p.config.PongWait))
		return nil
	})
	for {
		messageType, message, err := p.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("error:", err)
			}
			break
		}
		if messageType == websocket.CloseMessage {
			log.Println("closed:", p.ID)
			break
		}

		// 从消息中取出多条单个消息一一处理
		buf := bytes.NewReader(message)
		mlen := len(message)
		for i := 0; i < mlen; {
			msg, err := wire.ReadBytes(buf)
			if err != nil {
				log.Println(err)
				// no more message
				break
			}
			i = i + 4 + len(msg)
			go func(message []byte) {
				err = p.config.Listeners.OnMessage(msg)
				if err != nil {
					log.Println(err)
				}
			}(msg)
		}
	}
}

// 消息队列处理
func (p *Peer) outQueueHandler() {
	pendingMsgs := list.New()

	// We keep the waiting flag so that we know if we have a pending message
	waiting := false

	// To avoid duplication below.
	queuePacket := func(msg outMessage, list *list.List, waiting bool) bool {
		if !waiting {
			p.sendQueue <- msg
		} else {
			list.PushBack(msg)
		}
		// we are always waiting now.
		return true
	}
Loop:
	for {
		select {
		case msg, _ := <-p.outQueue:
			waiting = queuePacket(msg, pendingMsgs, waiting)
		case <-p.sendDone:
			next := pendingMsgs.Front()
			if next == nil {
				waiting = false
				continue
			}

			// Notify the handleWirte about the next item to
			// asynchronously send.
			val := pendingMsgs.Remove(next)
			p.sendQueue <- val.(outMessage)
		case <-p.quit:
			break Loop
		}
	}
	p.queueQuit <- struct{}{}

	// clean
	for next := pendingMsgs.Front(); next != nil; {
		pendingMsgs.Remove(next)
	}
}

func (p *Peer) outMessageHandler() {
	ticker := time.NewTicker(p.config.PingPeriod)
	defer func() {
		p.disconnect()
		ticker.Stop()
	}()
Loop:
	for {
		select {
		case outMessage := <-p.sendQueue:
			p.conn.SetWriteDeadline(time.Now().Add(p.config.WriteWait))

			w, err := p.conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				log.Println(err)
				return
			}
			wire.WriteBytes(w, outMessage.message)
			if err := w.Close(); err != nil {
				log.Println(err)
				return
			}

			if outMessage.done != nil {
				outMessage.done <- struct{}{}
			}

			p.sendDone <- struct{}{}
		case <-ticker.C:
			p.conn.SetWriteDeadline(time.Now().Add(p.config.WriteWait))
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println(err)
				break Loop
			}
		case <-p.queueQuit:
			p.conn.WriteMessage(websocket.CloseMessage, nil)
			break Loop
		}
	}
}

// SendMessage send a message to peer
func (p *Peer) SendMessage(message wire.Message, doneChan chan<- struct{}) error {
	buf := &bytes.Buffer{}
	err := wire.WriteMessage(buf, message)
	if err != nil {
		return err
	}
	p.PushMessage(buf.Bytes(), doneChan)
	return nil
}

// PushMessage 把消息写到队列中，等待处理
func (p *Peer) PushMessage(message []byte, doneChan chan<- struct{}) {
	if !p.IsConnected() {
		if doneChan != nil {
			go func() {
				doneChan <- struct{}{}
			}()
		}
		return
	}
	// log.Println("send message", message)
	p.outQueue <- outMessage{message: message, done: doneChan}
}

// Close close conn
func (p *Peer) Close() {
	if !p.IsConnected() {
		return
	}
	p.quit <- struct{}{}
}

//  断开连接
func (p *Peer) disconnect() {
	if !atomic.CompareAndSwapInt32(&p.connected, 1, 0) {
		return
	}
	p.conn.Close()
}

// IsConnected 判断连接是否正常
func (p *Peer) IsConnected() bool {
	return atomic.LoadInt32(&p.connected) == 1
}
