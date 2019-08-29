// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	secret = "xxx123456"
)

const sendurl = "http://192.168.0.188:8380/msg/send"

// MsgBody MsgBody
type MsgBody struct {
	Source string
	Dest   string
	Type   uint8
	Text   string
	Extra  string
}

func main() {
	wg := sync.WaitGroup{}
	var num = 1
	if len(os.Args) >= 2 {
		num, _ = strconv.Atoi(os.Args[1])
	}

	for index := 0; index < num; index++ {
		wg.Add(1)
		go func() {
			msg := MsgBody{
				Source: "/p/1/1/sys",
				Dest:   "/g/1/0/test",
				Type:   1,
				Text:   "{\"sportId\":1,\"goalTime\":35,\"league\":\"xxx\",\"homeTeam\":\"A\",\"visitingTeam\":\"B\",\"score\":\"2:0\",\"goalTeam\":1}",
			}

			d, _ := json.Marshal(msg)
			client := &http.Client{
				Timeout: time.Second * 5,
			}

			resp, err := client.Post(sendurl, "application/json", bytes.NewBuffer(d))

			if err != nil {
				fmt.Println(err)
				wg.Done()
				return
			}
			fmt.Println(resp.Status)
			wg.Done()
		}()
	}

	wg.Wait()
}
