// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ws-cluster/database"

	"github.com/ws-cluster/wire"
)

const (
	secret = "xxx123456"
)
const sendurl = "http://192.168.0.127:8380/msg/send"

func main() {
	wg := sync.WaitGroup{}

	for index := 0; index < 10; index++ {
		go func() {
			wg.Add(1)
			defer wg.Done()
			msg := database.ChatMsg{
				From:  "sys",
				Scope: wire.ScopeGroup,
				To:    "fb_bet_now_notify",
				Type:  1,
				Text:  "1276099",
			}

			d, _ := json.Marshal(msg)
			client := &http.Client{
				Timeout: time.Second * 5,
			}

			resp, err := client.Post(sendurl, "application/json", bytes.NewBuffer(d))

			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(resp.Status)
		}()
	}

	wg.Wait()

}
