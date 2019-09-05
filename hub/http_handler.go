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
	"time"

	"github.com/gorilla/websocket"
	"github.com/ws-cluster/wire"
)

// start http server ,this function must be in a routine
func httplisten(hub *Hub, conf *serverConfig) {

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

	log.Println("listen on ", conf.ListenHost)
	err := http.ListenAndServe(conf.ListenHost, nil)
	if err != nil {
		log.Println("ListenAndServe: ", err)
		return
	}
}

// 处理来自客户端节点的连接
func handleClientWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	addr := q.Get("addr") //  /p/domain/1/id
	nonce := q.Get("nonce")
	digest := q.Get("digest")
	offlineNotice := uint8(0)
	if q.Get("notice") == "1" {
		offlineNotice = uint8(1)
	}

	if addr == "" || nonce == "" || digest == "" {
		// 错误处理，断开
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// 校验digest及数据完整性
	if !checkDigest(hub.config.sc.Secret, fmt.Sprintf("%v%v", addr, nonce), digest) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	peerAddr, err := wire.ParseClientAddr(addr)
	if err != nil {
		log.Println("address error", addr)
		handleHTTPErr(w, err)
		return
	}

	// upgrade
	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		handleHTTPErr(w, err)
		return
	}

	clientPeer, err := newClientPeer(*peerAddr, r.RemoteAddr, offlineNotice, hub, conn)

	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	respchan := make(chan *Resp)
	// 注册节点到服务器
	hub.packetQueue <- &Packet{from: hub.Server.Addr, use: useForAddClientPeer, content: clientPeer, resp: respchan}
	resp := <-respchan
	if resp.Err != nil {
		handleHTTPErr(w, err)
		return
	}
	log.Printf("client %v@%v connected", addr, r.RemoteAddr)
	ack := wire.MakeEmptyHeaderMessage(wire.MsgTypeLoginAck, &wire.MsgLoginAck{
		RemoteAddr: r.RemoteAddr,
		LoginAt:    uint64(time.Now().UnixNano() / 1000000),
	})
	clientPeer.PushMessage(ack, nil)
}

var supgrader = &websocket.Upgrader{
	ReadBufferSize:  10240,
	WriteBufferSize: 10240,
}

// 处理来自服务器节点的连接
func handleServerWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	addrstr := r.Header.Get("addr")
	ServerURL, err := url.Parse(r.Header.Get("server_url"))
	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	PeerURL, err := url.Parse(r.Header.Get("peer_url"))
	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	digest := r.Header.Get("digest")

	serverAddr, err := wire.ParseServerAddr(addrstr)
	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	// 校验digest及数据完整性
	if !checkDigest(hub.config.sc.Secret, addrstr, digest) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	conn, err := supgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	serverPeer, err := bindServerPeer(hub, conn, &Server{
		Addr:               *serverAddr,
		Secret:             hub.config.sc.Secret,
		AdvertiseClientURL: PeerURL,
		AdvertiseServerURL: ServerURL,
	}, r.RemoteAddr)
	if err != nil {
		log.Println(err)
		return
	}

	respchan := make(chan *Resp)
	// 注册节点到服务器
	hub.packetQueue <- &Packet{from: hub.Server.Addr, use: useForAddServerPeer, content: serverPeer, resp: respchan}

	resp := <-respchan
	if resp.Err != nil {
		handleHTTPErr(w, err)
		return
	}

	log.Printf("server %v connected", serverAddr.String())
}

// MsgBody MsgBody
type MsgBody struct {
	Source string
	Dest   string
	Type   uint8
	Text   string
	Extra  string
}

// 处理 http 过来的消息发送
func httpSendMsgHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	var body MsgBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	fmt.Println("httpSendMsg ", r.RemoteAddr, body.Dest)

	msg := wire.MakeEmptyHeaderMessage(wire.MsgTypeChat, &wire.Msgchat{
		Text:  body.Text,
		Type:  body.Type,
		Extra: body.Extra,
	})

	source, err := wire.ParseAddr(body.Source)
	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	dest, err := wire.ParseAddr(body.Dest)
	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	msg.Header.Source = *source
	msg.Header.Dest = *dest
	respchan := make(chan *Resp)
	hub.packetQueue <- &Packet{from: hub.Server.Addr, use: useForRelayMessage, content: msg, resp: respchan}
	resp := <-respchan

	if resp.Status == wire.MsgStatusOk {
		fmt.Fprint(w, "ok")
	} else {
		fmt.Fprint(w, "fail")
		w.WriteHeader(http.StatusExpectationFailed)
	}
}

// 处理 http 过来的消息发送
func httpQueryClientOnlineHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	addrstr := r.URL.Query().Get("addr")
	addr, err := wire.ParseAddr(addrstr)
	if err != nil {
		fmt.Fprint(w, err.Error())
		w.WriteHeader(http.StatusBadRequest)
	}
	respchan := make(chan *Resp)

	msg := wire.MakeEmptyHeaderMessage(wire.MsgTypeQueryClient, &wire.MsgQueryClient{
		Peer: *addr,
	})
	msg.Header.Dest = hub.Server.Addr
	hub.packetQueue <- &Packet{from: hub.Server.Addr, use: useForRelayMessage, content: msg, resp: respchan}

	resp := <-respchan

	qu := resp.Body.(*wire.MsgQueryClientResp)
	fmt.Fprint(w, qu.LoginAt)
}

func checkDigest(secret, text, digest string) bool {
	h := md5.New()
	io.WriteString(h, text)
	io.WriteString(h, secret)
	return digest == hex.EncodeToString(h.Sum(nil))
}

func handleHTTPErr(w http.ResponseWriter, err error) {
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
	log.Println(err)
}
