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
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/ws-cluster/wire"

	"github.com/gorilla/websocket"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/peer"
)

const (
	secret = "xxx123456"
)

func newPeer(addr, serverhost, secret string, OnMessage func(message *wire.Message) error, OnDisconnect func() error) (*peer.Peer, error) {
	nonce := fmt.Sprint(time.Now().UnixNano())
	h := md5.New()
	io.WriteString(h, addr)
	io.WriteString(h, nonce)
	io.WriteString(h, secret)

	query := fmt.Sprintf("addr=%v&nonce=%v&digest=%v", addr, nonce, hex.EncodeToString(h.Sum(nil)))

	u := url.URL{Scheme: "ws", Host: serverhost, Path: "/client", RawQuery: query}
	log.Printf("connecting to %s", u.String())

	client := database.Client{
		ID: addr,
	}

	peer := peer.NewPeer(client.ID, "", &peer.Config{
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
	addr       *wire.Addr
	serverHost string
	secret     string
	message    chan *wire.Message
}

func newClientPeer(secret, serverHost string, clientAddr *wire.Addr, autoConn bool, msg chan *wire.Message) (*ClientPeer, error) {

	clientPeer := &ClientPeer{
		AutoConn:   autoConn,
		serverHost: serverHost,
		secret:     secret,
		message:    msg,
		addr:       clientAddr,
	}
	if err := clientPeer.newPeer(); err != nil {
		return nil, err
	}

	return clientPeer, nil
}

// OnMessage OnMessage
func (p *ClientPeer) OnMessage(message *wire.Message) error {
	p.message <- message
	return nil
}

func (p *ClientPeer) newPeer() error {
	peer, err := newPeer(p.addr.String(), p.serverHost, p.secret, p.OnMessage, p.OnDisconnect)
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
				break
			}
		}
	}
	return nil
}

func sendtoclient(peer *ClientPeer, to *wire.Addr) {
	done := make(chan error)
	msg := wire.MakeEmptyHeaderMessage(wire.MsgTypeChat, &wire.Msgchat{
		Type: 1,
		Text: "hello",
	})
	msg.Header.Source = *peer.addr
	msg.Header.Dest = *to
	peer.PushMessage(msg, done)
	<-done
}

var wshosts = []string{"192.168.0.188:8380", "192.168.0.188:8380"}

// var wshosts = []string{"tapi.zhiqiu666.com:8098", "192.168.0.188:8380"}
var peerNum = 1

func main() {
	// listen sys.exit
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)

	if len(os.Args) >= 2 {
		peerNum, _ = strconv.Atoi(os.Args[1])
	}
	// peers := make(map[string]*ClientPeer, peerNum)
	addpeer := make(chan *ClientPeer, 100)
	msgchan := make(chan *wire.Message, 100)
	intervalMsgNum := 0
	totalMsgNum := 0
	totalPeerNum := 0
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-addpeer:
				totalPeerNum++
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
	t1 := time.Now()
	for index := 0; index < peerNum; index++ {
		ws.Add(1)
		go func(i int) {
			wshost := wshosts[i%2]
			addr, _ := wire.NewAddr(wire.AddrPeer, 0, wire.DevicePhone, fmt.Sprintf("client_%v", i))
			peer, err := newClientPeer(secret, wshost, addr, false, msgchan)
			if err != nil {
				log.Println(err)
			} else {
				addpeer <- peer
			}

			ws.Done()
		}(index)
	}
	ws.Wait()

	t2 := time.Now()
	log.Printf("login client[%v], cost time: %v", peerNum, t2.Sub(t1))

	<-sc
}
