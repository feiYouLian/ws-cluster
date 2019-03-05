// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ws-cluster/hub"

	"github.com/ws-cluster/wire"
)

const (
	secret = "xxx123456"
)

func main() {

	msg := hub.SendMessageBody{
		From:  "sys",
		Scope: wire.ScopeGroup,
		To:    "fb_bet_now_notify",
		Type:  1,
		Text:  "hello",
	}

	d, _ := json.Marshal(msg)
	resp, err := http.Post("http://localhost:8380/sendMsg", "application/json", bytes.NewBuffer(d))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(resp.Status)
}
