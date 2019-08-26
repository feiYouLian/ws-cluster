// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ws-cluster/wire"

	"github.com/gorilla/websocket"
	"github.com/ws-cluster/peer"
)

const (
	secret = "xxx123456"
)

func newPeer(addr wire.Addr, serverhost, secret string, OnMessage func(message *wire.Message) error, OnDisconnect func() error) (*peer.Peer, error) {
	nonce := fmt.Sprint(time.Now().UnixNano())
	h := md5.New()
	io.WriteString(h, addr.String())
	io.WriteString(h, nonce)
	io.WriteString(h, secret)

	query := fmt.Sprintf("addr=%v&nonce=%v&digest=%v", addr.String(), nonce, hex.EncodeToString(h.Sum(nil)))

	u := url.URL{Scheme: "ws", Host: serverhost, Path: "/client", RawQuery: query}
	log.Printf("connecting to %s", u.String())

	peer := peer.NewPeer(addr, "", &peer.Config{
		Listeners: &peer.MessageListeners{
			OnMessage:    OnMessage,
			OnDisconnect: OnDisconnect,
		},
	})

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Println("dial:", err)
		return nil, err
	}
	peer.SetConnection(conn)

	return peer, nil
}

// ClientPeer ClientPeer
type ClientPeer struct {
	*peer.Peer
	// AutoConn 是否自动重连
	AutoConn   bool
	addr       wire.Addr
	serverHost string
	secret     string
	message    chan *wire.Message
	connet     chan *ClientPeer
	disconnet  chan *ClientPeer
}

func newClientPeer(secret, serverHost string, clientAddr wire.Addr, msg chan *wire.Message, connet, disconnet chan *ClientPeer) (*ClientPeer, error) {

	clientPeer := &ClientPeer{
		AutoConn:   false,
		serverHost: serverHost,
		secret:     secret,
		message:    msg,
		addr:       clientAddr,
		connet:     connet,
		disconnet:  disconnet,
	}
	if err := clientPeer.newPeer(); err != nil {
		return nil, err
	}

	return clientPeer, nil
}

// OnMessage OnMessage
func (p *ClientPeer) OnMessage(message *wire.Message) error {
	if message.Header.Command == wire.MsgTypeLoginAck {
		p.connet <- p
		return nil
	}
	p.message <- message

	if message.Header.Command == wire.MsgTypeChat {
		if message.Header.Dest.Type() == wire.AddrPeer { // peer to peer
			ackmessage := wire.MakeEmptyHeaderMessage(wire.MsgTypeChatResp, &wire.MsgChatResp{
				State: wire.AckSent,
			})
			ackmessage.Header.Source = p.addr
			ackmessage.Header.Dest = message.Header.Source
			ackmessage.Header.AckSeq = message.Header.Seq
			p.PushMessage(ackmessage, nil)
		}
	}

	return nil
}

func (p *ClientPeer) newPeer() error {
	peer, err := newPeer(p.addr, p.serverHost, p.secret, p.OnMessage, p.OnDisconnect)
	if err != nil {
		log.Println(err)
		return err
	}
	p.Peer = peer
	return nil
}

// OnDisconnect OnDisconnect
func (p *ClientPeer) OnDisconnect() error {
	if p.AutoConn {
		for i := 0; i < 60; i++ {
			time.Sleep(time.Second * 3)
			if err := p.newPeer(); err == nil {
				return nil
			}
		}
	}
	// log.Println(p.Addr.String(), "disconnect")
	p.disconnet <- p

	return nil
}

func sendtoclient(peer *ClientPeer, to wire.Addr) {
	// done := make(chan error)
	msg := wire.MakeEmptyHeaderMessage(wire.MsgTypeChat, &wire.Msgchat{
		Type: 1,
		Text: "hello",
	})
	msg.Header.Source = peer.addr
	msg.Header.Dest = to
	peer.PushMessage(msg, nil)
	// <-done
}

var wshosts = []string{"192.168.0.188:8380", "192.168.0.188:8380"}

// var wshosts = []string{"tapi.zhiqiu666.com:8098", "192.168.0.188:8380"}
var peerNum = 1

func main() {
	// listen sys.exit

	if len(os.Args) >= 2 {
		peerNum, _ = strconv.Atoi(os.Args[1])
	}
	// peers := make(map[string]*ClientPeer, peerNum)

	msgchan := make(chan *wire.Message, 100)
	connetchan := make(chan *ClientPeer, 100)
	disconnetchan := make(chan *ClientPeer, 100)
	var quit = make(chan bool)

	intervalMsgNum := 0
	totalMsgNum := 0
	totalPeerNum := 0

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	t1 := time.Now()

	testgroup, _ := wire.NewGroupAddr(1, "test")
	go func() {
		for {
			select {
			case peer := <-connetchan:
				totalPeerNum++
				if totalPeerNum == peerNum {
					t2 := time.Now()
					log.Printf("login client[%v], cost time: %v", peerNum, t2.Sub(t1))
				}

				msg := wire.MakeEmptyHeaderMessage(wire.MsgTypeGroupInOut, &wire.MsgGroupInOut{
					InOut:  wire.GroupIn,
					Groups: []wire.Addr{*testgroup},
				})
				msg.Header.Source = peer.Addr
				peer.PushMessage(msg, nil)

			case <-disconnetchan:
				totalPeerNum--
				if totalPeerNum == 0 {
					quit <- true
				}
			case <-msgchan:
				intervalMsgNum++
				totalMsgNum++
			case <-ticker.C:
				if intervalMsgNum > 0 {
					log.Printf("1秒内收到消息数据：%v,总接收消息数：%v,总节点数：%v", intervalMsgNum, totalMsgNum, totalPeerNum)
				}
				intervalMsgNum = 0
			}
		}
	}()

	ws := sync.WaitGroup{}

	for index := 0; index < peerNum; index++ {
		ws.Add(1)
		go func(i int) {
			wshost := wshosts[i%2]
			addr, _ := wire.NewAddr(wire.AddrPeer, 0, wire.DevicePhone, fmt.Sprintf("client_%v", i))
			_, err := newClientPeer(secret, wshost, *addr, msgchan, connetchan, disconnetchan)
			if err != nil {
				log.Println(err)
			}
			ws.Done()
		}(index)
	}
	ws.Wait()
	log.Println("new peer finish")
	<-quit
}
