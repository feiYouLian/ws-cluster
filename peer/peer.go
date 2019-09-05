package peer

import (
	"bytes"
	"container/list"
	"errors"
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
	defaultPongWait = 20 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	defaultPingPeriod = 10 * time.Second

	// Maximum message size allowed from peer.
	defaultMaxMessageSize = 1024
)

// ErrPeerNotOpen peer has closed, pushmessage failed
var ErrPeerNotOpen = errors.New("peer not open")

// MessageListeners 消息监听
type MessageListeners struct {
	// OnGetAddr is invoked when a peer receives a getaddr bitcoin message.
	OnMessage func(msg *wire.Message) error

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

const (
	packetUseForMessage = uint8(1)
	packetUseForClose   = uint8(9)
)

type packet struct {
	use     uint8 // 9 exit  1 message out
	content interface{}
	done    chan<- error
}

// type quitMessage struct {
// 	done chan struct{}
// }

// Peer 节点封装了 websocket 通信底层接口
type Peer struct {
	Addr       wire.Addr
	RemoteAddr string // ip:port
	config     *Config
	conn       *websocket.Conn
	outQueue   chan packet
	sendQueue  chan packet
	sendDone   chan struct{}
	// quit          chan quitMessage
	connclosed    chan struct{}
	timeConnected time.Time

	connected int32 // 0 unconnected 1 connected 2 closing
	autoSeq   uint32
}

// NewPeer 创建一个新的节点
func NewPeer(addr wire.Addr, RemoteAddr string, config *Config) *Peer {
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
		Addr:       addr,
		RemoteAddr: RemoteAddr,
		config:     config,
		outQueue:   make(chan packet, 1),
		sendQueue:  make(chan packet, 1),
		sendDone:   make(chan struct{}, 1),
		// quit:       make(chan quitMessage, 1),
		connclosed: make(chan struct{}, 1),
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
	go p.packetQueueHandler()
	go p.packetHandler()
	// log.Printf("peer %v started", p.ID)
}

func (p *Peer) inMessageHandler() {
	defer func() {
		p.connectionClosed()
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
			// if websocket.IsCloseError(err,websocket.E) {
			// }
			log.Println(p.Addr.String(), err)

			if websocket.IsUnexpectedCloseError(err, websocket.CloseMessageTooBig) &&
				p.Addr.Type() == wire.AddrServer {
				continue
			}

			break
		}
		if messageType == websocket.CloseMessage {
			log.Println("closed:", p.Addr.String())
			break
		}
		if len(message) == 0 {
			continue
		}
		// 从消息中取出多条单个消息一一处理
		buf := bytes.NewReader(message)
		for {
			msg := new(wire.Message)
			err := msg.Decode(buf)
			if err != nil { // read EOF,no more message
				break
			}
			if p.Addr.Type() == wire.AddrClient {
				msg.Header.Source = p.Addr // set source
			}

			go func() {
				err = p.config.Listeners.OnMessage(msg)
				if err != nil {
					log.Println("peer.go line:162 ", p.Addr.String(), err)
				}
			}()
		}
	}
}

func (p *Peer) packetHandler() {
	ticker := time.NewTicker(p.config.PingPeriod)
	defer func() {
		ticker.Stop()
	}()

	for {
		select {
		case packet := <-p.sendQueue:
			var err error
			if packet.use == packetUseForMessage {
				err = p.writeMessage(packet.content.(*wire.Message))
			} else if packet.use == packetUseForClose { // close connection
				p.closeConnect() //actively close the connection
				return
			}

			packet.done <- err
			p.sendDone <- struct{}{}
		case <-ticker.C:
			p.conn.SetWriteDeadline(time.Now().Add(p.config.WriteWait))
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				p.connectionClosed()
				return
			}
		}
	}
}

func (p *Peer) writeMessage(message *wire.Message) error {
	header := message.Header
	if header.Seq == 0 { // message sender set a Seq
		p.autoSeq++
		header.Seq = p.autoSeq
	}

	p.conn.SetWriteDeadline(time.Now().Add(p.config.WriteWait))

	w, err := p.conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return err
	}

	err = message.Encode(w)
	if err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}
	return nil
}

// 消息队列处理
func (p *Peer) packetQueueHandler() {
	pendingMsgs := list.New()

	// We keep the waiting flag so that we know if we have a pending message
	waiting := false

	// To avoid duplication below.
	queuePacket := func(msg packet, list *list.List, waiting bool) bool {
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
			p.sendQueue <- val.(packet)
		case <-p.connclosed: //connection has closed
			break Loop
		}
	}

	// clean
	for next := pendingMsgs.Front(); next != nil; {
		pendingMsgs.Remove(next)
	}
}

// PushMessage 把消息写到队列中，等待处理。如果连接已经关系，消息会被丢掉
func (p *Peer) PushMessage(message *wire.Message, doneChan chan error) {
	if doneChan == nil { //throw err
		doneChan = make(chan error, 1)
		go func() {
			<-doneChan
		}()
	}

	if !p.IsConnected() {
		doneChan <- ErrPeerNotOpen
		return
	}

	p.outQueue <- packet{use: packetUseForMessage, content: message, done: doneChan}
}

// Close Close peer conn
func (p *Peer) Close() {
	if !p.IsConnected() {
		return
	}
	if !atomic.CompareAndSwapInt32(&p.connected, 1, 2) { //closing
		return
	}
	done := make(chan error, 1)
	go func() {
		<-done
	}()
	p.outQueue <- packet{use: packetUseForClose, content: nil, done: done}
}

//  actively close the connection
func (p *Peer) closeConnect() {
	if !atomic.CompareAndSwapInt32(&p.connected, 2, 0) {
		return
	}
	p.conn.Close()
	p.connclosed <- struct{}{}
}

// connection has been closed by other side
func (p *Peer) connectionClosed() {
	if !atomic.CompareAndSwapInt32(&p.connected, 1, 2) {
		return
	}
	p.connclosed <- struct{}{}

	err := p.config.Listeners.OnDisconnect()
	if err != nil {
		log.Println(err)
	}
}

// IsConnected 判断连接是否正常
func (p *Peer) IsConnected() bool {
	return atomic.LoadInt32(&p.connected) == 1
}
