// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ws-cluster/database"

	"github.com/ws-cluster/wire"
)

const (
	secret = "xxx123456"
)

func main() {

	msg := database.ChatMsg{
		From:  "sys",
		Scope: wire.ScopeGroup,
		To:    "fb_bet_now_notify",
		Type:  1,
		Text:  "1322324",
	}

	d, _ := json.Marshal(msg)
	client := &http.Client{
		Timeout: time.Second * 3,
	}

	resp, err := client.Post("http://192.168.0.127:8380/sendMsg", "application/json", bytes.NewBuffer(d))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(resp.Status)
}
