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
	"time"

	"github.com/ws-cluster/wire"

	"github.com/gorilla/websocket"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/peer"
)

const (
	secret = "xxx123456"
)

func newPeer(clientID, addr, secret string, OnMessage func(message []byte) error, OnDisconnect func() error) (*peer.Peer, error) {
	nonce := fmt.Sprint(time.Now().UnixNano())
	h := md5.New()
	io.WriteString(h, clientID)
	io.WriteString(h, nonce)
	io.WriteString(h, secret)

	query := fmt.Sprintf("id=%v&nonce=%v&digest=%v", clientID, nonce, hex.EncodeToString(h.Sum(nil)))

	u := url.URL{Scheme: "ws", Host: addr, Path: "/client", RawQuery: query}
	log.Printf("connecting to %s", u.String())

	client := database.Client{
		ID: clientID,
	}

	peer := peer.NewPeer(fmt.Sprintf("C%v", client.ID), &peer.Config{
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
	clientID   string
	serverAddr string
	secret     string
	message    chan []byte
}

func newClientPeer(secret, clientID, addr string, autoConn bool, msg chan []byte) (*ClientPeer, error) {
	clientPeer := &ClientPeer{
		AutoConn:   autoConn,
		clientID:   clientID,
		serverAddr: addr,
		secret:     secret,
		message:    msg,
	}
	if err := clientPeer.newPeer(); err != nil {
		return nil, err
	}

	return clientPeer, nil
}

// OnMessage OnMessage
func (p *ClientPeer) OnMessage(message []byte) error {
	p.message <- message
	return nil
}

func (p *ClientPeer) newPeer() error {
	peer, err := newPeer(p.clientID, p.serverAddr, p.secret, p.OnMessage, p.OnDisconnect)
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

func sendtoclient(peer *ClientPeer, to string) {
	done := make(chan struct{})
	msg, _ := wire.MakeEmptyMessage(&wire.MessageHeader{ID: uint32(time.Now().Unix()), Msgtype: wire.MsgTypeChat, Scope: wire.ScopeClient, To: to})
	chatMsg := msg.(*wire.Msgchat)
	chatMsg.From = peer.clientID
	chatMsg.Type = 1
	chatMsg.Text = fmt.Sprint("hello")
	peer.SendMessage(chatMsg, done)
	<-done
}

var wshosts = []string{"192.168.0.155:8380", "192.168.0.188:8380"}

func sendRobot(peerNum int) {
	msgchan := make(chan []byte, 100)
	quit := make(chan bool)
	ackNum := 0
	totalNum := 0
	ticker := time.NewTicker(time.Second)

	sendNum := peerNum * 10

	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-msgchan:
				ackNum++
				totalNum++
			case <-ticker.C:
				log.Printf("1秒内收到ACK消息数据:%v, 总收到ACK消息数:%v", ackNum, totalNum)
				ackNum = 0
				if totalNum == sendNum {
					quit <- true
				}
			}
		}
	}()

	syspeer, err := newClientPeer(secret, "sys", wshosts[0], false, msgchan)
	if err != nil {
		log.Println(err)
		return
	}

	t1 := time.Now()
	for index := 0; index < sendNum; index++ {
		sendtoclient(syspeer, fmt.Sprintf("client_%v", index%peerNum))
	}
	t2 := time.Now()
	log.Printf("send message[%v], cost time: %v", sendNum, t2.Sub(t1))
	<-quit
}

func main() {
	// listen sys.exit
	var peerNum = 1
	if len(os.Args) >= 2 {
		peerNum, _ = strconv.Atoi(os.Args[1])
	}

	sendRobot(peerNum)
}
