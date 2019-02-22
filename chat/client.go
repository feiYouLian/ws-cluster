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
}

func newClientPeer(secret, clientID, addr string, autoConn bool) (*ClientPeer, error) {
	clientPeer := &ClientPeer{
		AutoConn:   autoConn,
		clientID:   clientID,
		serverAddr: addr,
		secret:     secret,
	}
	if err := clientPeer.newPeer(); err != nil {
		return nil, err
	}

	return clientPeer, nil
}

// OnMessage OnMessage
func (p *ClientPeer) OnMessage(message []byte) error {
	log.Println("OnMessage", message)
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

func robot(from, to string, quit chan os.Signal) {
	peer, err := newClientPeer(secret, from, "192.168.0.188:8380", true)
	if err != nil {
		log.Println(err)
		return
	}
	if to != "" {
		ws := sync.WaitGroup{}
		// 测试发送100条消息时间
		t1 := time.Now().UnixNano()
		for index := uint32(0); index < 1; index++ {
			ws.Add(1)
			go func(i uint32) {
				done := make(chan struct{})
				msg, _ := wire.MakeEmptyMessage(&wire.MessageHeader{ID: i, Msgtype: wire.MsgTypeChat, Scope: wire.ScopeClient, To: to})
				chatMsg := msg.(*wire.Msgchat)
				chatMsg.From = from
				chatMsg.Type = 1
				chatMsg.Text = fmt.Sprint("hello, im robot", i)
				peer.SendMessage(chatMsg, done)
				<-done
				ws.Done()
			}(index)
		}
		ws.Wait()
		t2 := time.Now().UnixNano()
		log.Println("cost time:", (t2-t1)/1000)
	}
	done := make(chan struct{})
	msg2, _ := wire.MakeEmptyMessage(&wire.MessageHeader{ID: 2, Msgtype: wire.MsgTypeChat, Scope: wire.ScopeGroup, To: "fb_score_notify"})
	chatMsg2 := msg2.(*wire.Msgchat)
	chatMsg2.From = from
	chatMsg2.Type = 1
	chatMsg2.Text = "{'sport_id':1,'goalTime':323232434,'homeTeam':'A','vistingTeam':'B','score':'2:0','goalTeam':1}"

	peer.SendMessage(chatMsg2, done)
	<-done

	<-quit
	// peer.Peer.Close()

}

func main() {
	// listen sys.exit
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	var from, to string
	if len(os.Args) >= 2 {
		from = os.Args[1]
	}
	if len(os.Args) >= 3 {
		to = os.Args[2]
	}

	robot(from, to, sc)
}
