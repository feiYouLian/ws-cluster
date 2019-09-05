// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/ws-cluster/peer"
	"github.com/ws-cluster/wire"
)

func Test_sendtoclient(t *testing.T) {
	msgchan := make(chan *wire.Message, 100)
	connetchan := make(chan *peer.Peer, 100)
	disconnetchan := make(chan *peer.Peer)
	quit := make(chan bool)
	ackNum := 0
	totalNum := 0
	ticker := time.NewTicker(time.Second)

	peerNum := 1
	sendNum := peerNum
	sendGroupMsgNum := 1000

	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-disconnetchan:
				quit <- true
			case <-connetchan:
				log.Println("login success")
			case message := <-msgchan:
				if message.Header.AckSeq > 0 {
					ackNum++
					continue
				}
				totalNum++
				// log.Println(" header", message.Header.String())
			case <-ticker.C:
				if ackNum > 0 {
					log.Printf("1秒内收到ACK 消息数据:%v, 总收到ACK消息数:%v", ackNum, totalNum)
				}
				ackNum = 0
				if totalNum == (sendNum + sendGroupMsgNum) {
					quit <- true
				}
			}
		}
	}()
	sysaddr, _ := wire.NewAddr(wire.AddrPeer, 0, wire.DevicePhone, "sys")

	syspeer, err := newClientPeer(secret, wshosts[0], *sysaddr, msgchan, connetchan, disconnetchan)
	if err != nil {
		log.Println(err)
		return
	}

	t1 := time.Now()
	for index := 0; index < sendNum; index++ {
		addr, _ := wire.NewAddr(wire.AddrPeer, 0, wire.DevicePhone, fmt.Sprintf("client_%v", index%peerNum))
		sendtoclient(syspeer.Peer, *addr)
		// time.Sleep(time.Second)
	}

	testgroup, _ := wire.NewGroupAddr(1, "test")
	for index := 0; index < sendGroupMsgNum; index++ {
		sendtoclient(syspeer.Peer, *testgroup)
		time.Sleep(time.Millisecond * 10)
	}
	t2 := time.Now()
	log.Printf("send message[%v], cost time: %v", sendNum+sendGroupMsgNum, t2.Sub(t1))
	<-quit
}
