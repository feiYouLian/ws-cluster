// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"testing"
	"time"
)

func Test_sendtoclient(t *testing.T) {
	msgchan := make(chan []byte, 100)
	quit := make(chan bool)
	ackNum := 0
	totalNum := 0
	ticker := time.NewTicker(time.Second)

	peerNum := 100
	sendNum := peerNum * 10

	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-msgchan:
				ackNum++
				totalNum++
			case <-ticker.C:
				log.Printf("1秒内收到ACK 消息数据:%v, 总收到ACK消息数:%v", ackNum, totalNum)
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
		// time.Sleep(time.Second)
	}
	t2 := time.Now()
	log.Printf("send message[%v], cost time: %v", sendNum, t2.Sub(t1))
	<-quit
}
