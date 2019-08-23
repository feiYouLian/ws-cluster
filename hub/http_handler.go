package hub

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/ws-cluster/config"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/wire"
)

// start http server ,this function must be in a routine
func httplisten(hub *Hub, conf *config.ServerConfig) {

	// regist a service for client
	http.HandleFunc("/client", func(w http.ResponseWriter, r *http.Request) {
		handleClientWebSocket(hub, w, r)
	})
	// regist a service for server
	http.HandleFunc("/server", func(w http.ResponseWriter, r *http.Request) {
		handleServerWebSocket(hub, w, r)
	})

	http.HandleFunc("/msg/send", func(w http.ResponseWriter, r *http.Request) {
		httpSendMsgHandler(hub, w, r)
	})

	http.HandleFunc("/q/online", func(w http.ResponseWriter, r *http.Request) {
		httpQueryClientOnlineHandler(hub, w, r)
	})

	log.Println("listen on ", fmt.Sprintf("%s:%d", conf.ListenIP, conf.ListenPort))
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", conf.ListenIP, conf.ListenPort), nil)

	if err != nil {
		log.Println("ListenAndServe: ", err)
		return
	}
}

// 处理来自客户端节点的连接
func handleClientWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	addr := q.Get("addr") //  /p/domain/id
	nonce := q.Get("nonce")
	digest := q.Get("digest")

	if addr == "" || nonce == "" || digest == "" {
		// 错误处理，断开
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// 校验digest及数据完整性
	if !checkDigest(hub.config.Server.Secret, fmt.Sprintf("%v%v", addr, nonce), digest) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// upgrade
	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		handleHTTPErr(w, err)
		return
	}

	peerAddr, err := wire.NewPeerAddr(addr)
	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	peerID := fmt.Sprintf("%v@%v", addr, r.RemoteAddr)

	log.Printf("client %v connecting ", peerID)
	clientPeer, err := newClientPeer(*peerAddr, r.RemoteAddr, hub, conn)

	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	errchan := make(chan error, 0)
	// 注册节点到服务器
	hub.packetQueue <- &Packet{from: hub.Server.Addr, use: useForAddClientPeer, content: clientPeer, err: errchan}

	err = <-errchan
	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	log.Printf("client %v connected", peerID)
	ack := wire.MakeEmptyHeaderMessage(wire.MsgTypeLoginAck, &wire.MsgLoginAck{PeerID: peerID})
	clientPeer.PushMessage(ack, nil)
}

// 处理来自服务器节点的连接
func handleServerWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	addrstr := r.Header.Get("addr")
	URL, err := url.Parse(r.Header.Get("url"))
	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	digest := r.Header.Get("digest")

	serverAddr, err := wire.NewServerAddr(addrstr)
	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	// 校验digest及数据完整性
	if !checkDigest(hub.config.Server.Secret, fmt.Sprintf("%v%v", addrstr, URL.String()), digest) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	serverPeer, err := bindServerPeer(hub, conn, &Server{
		Addr:   *serverAddr,
		URL:    URL,
		Secret: hub.config.Server.Secret,
	}, r.RemoteAddr)
	if err != nil {
		log.Println(err)
		return
	}

	errchan := make(chan error, 0)
	// 注册节点到服务器
	hub.packetQueue <- &Packet{from: hub.Server.Addr, use: useForAddServerPeer, content: serverPeer, err: errchan}

	err = <-errchan
	if err != nil {
		handleHTTPErr(w, err)
		return
	}

	log.Printf("server %v connected", serverAddr.String())
}

// 处理 http 过来的消息发送
func httpSendMsgHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	var body database.ChatMsg
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// fmt.Println("httpSendMsg ", r.RemoteAddr, body.To, body.Text)

	msg := wire.MakeEmptyHeaderMessage(wire.MsgTypeChat, &wire.Msgchat{
		Text:  body.Text,
		Type:  body.Type,
		Extra: body.Extra,
	})

	source, _ := wire.NewAddr(wire.AddrPeer, body.FromDomain, wire.DevicePhone, body.From)
	dest, _ := wire.NewAddr(wire.AddrPeer, body.FromDomain, wire.DevicePhone, body.To)
	msg.Header.Source = *source
	msg.Header.Dest = *dest
	hub.packetQueue <- &Packet{from: hub.Server.Addr, use: useForRelayMessage, content: msg}
	fmt.Fprint(w, "ok")
}

// 处理 http 过来的消息发送
func httpQueryClientOnlineHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	addrstr := r.URL.Query().Get("addr")
	addr, err := wire.NewPeerAddr(addrstr)
	if err != nil {
		fmt.Fprint(w, err.Error())
		w.WriteHeader(http.StatusBadRequest)
	}
	respchan := make(chan *wire.Message)

	msg := wire.MakeEmptyHeaderMessage(wire.MsgTypeQueryClient, &wire.MsgQueryClient{
		Peer: *addr,
	})
	msg.Header.Dest = hub.Server.Addr
	hub.packetQueue <- &Packet{from: hub.Server.Addr, use: useForRelayMessage, content: msg, resp: respchan}

	message := <-respchan

	resp := message.Body.(*wire.MsgQueryClientResp)
	fmt.Fprint(w, resp.LoginAt)
}

func checkDigest(secret, text, digest string) bool {
	h := md5.New()
	io.WriteString(h, text)
	io.WriteString(h, secret)
	return digest == hex.EncodeToString(h.Sum(nil))
}

func handleHTTPErr(w http.ResponseWriter, err error) {
	fmt.Fprint(w, err.Error())
	w.WriteHeader(http.StatusBadRequest)
}
