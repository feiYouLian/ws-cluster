package hub

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

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

	listenIP := conf.Addr
	log.Println("listen on ", fmt.Sprintf("%s:%d", listenIP, conf.Listen))
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", listenIP, conf.Listen), nil)

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
	log.Println("client RemoteAddr:", r.RemoteAddr)
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

	clientPeer, err := newClientPeer(peerAddr, hub, conn, &database.Client{
		ID:       addr,
		PeerID:   fmt.Sprintf("%v@%v", addr, r.RemoteAddr),
		ServerID: hub.ServerID,
		LoginAt:  uint32(time.Now().Unix()),
	})

	if err != nil {
		handleHTTPErr(w, err)
		return
	}
	// 注册节点到服务器
	hub.register <- &addPeer{peer: clientPeer, done: nil}
	log.Printf("client %v connecting from %v", addr, r.RemoteAddr)
}

// 处理来自服务器节点的连接
func handleServerWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	ID, _ := strconv.ParseUint(r.Header.Get("id"), 0, 64)
	IP := r.Header.Get("ip")
	Port, _ := strconv.Atoi(r.Header.Get("port"))
	digest := r.Header.Get("digest")

	if ID == 0 || IP == "" || Port == 0 {
		// 错误处理，断开
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// 校验digest及数据完整性
	if !checkDigest(hub.config.Server.Secret, fmt.Sprintf("%v%v%v", ID, IP, Port), digest) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	serverPeer, err := bindServerPeer(hub, conn, &database.Server{
		ID:   ID,
		IP:   IP,
		Port: Port,
	})
	if err != nil {
		log.Println(err)
		return
	}

	hub.register <- &addPeer{peer: serverPeer, done: nil}

	log.Printf("server %v connecting from %v", ID, r.RemoteAddr)
}

// 处理 http 过来的消息发送
func httpSendMsgHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	var body database.ChatMsg
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// fmt.Println("httpSendMsg ", r.RemoteAddr, body.To, body.Text)

	msg, err := wire.MakeEmptyHeaderMessage(wire.MsgTypeChat, &wire.Msgchat{
		Text:  body.Text,
		Type:  body.Type,
		Extra: body.Extra,
	})
	if err != nil {
		fmt.Fprint(w, err.Error())
		return
	}
	msg.Header.Source, _ = wire.NewAddr(wire.AddrPeer, body.FromDomain, body.From)
	msg.Header.Dest, _ = wire.NewAddr(wire.AddrPeer, body.FromDomain, body.From)

	hub.msgQueue <- &Msg{from: clientFlag, message: msg}
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
	if p, ok := hub.clientPeers[*addr]; ok {
		fmt.Fprint(w, p.entity.LoginAt)
		return
	}
	fmt.Fprint(w, 0)
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
