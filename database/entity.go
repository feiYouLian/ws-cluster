package database

import (
	"time"
)

// Client 是一个具体的客户端身份对象
type Client struct {
	ID       string
	PeerID   string //全局唯一id.
	ServerID uint64
	LoginAt  uint32 //second
}

// Group 代表一个群，发往这个群的消息会转发给所有在此群中的用户。
// 聊天室不保存
type Group struct {
	Name    string
	Clients map[string]bool
}

// Server 服务器对象
type Server struct {
	// ID 服务器Id, 自动生成，缓存到file 中，重启时 ID 不变
	ID   uint64
	IP   string
	Port int
	// ClientNum 在线人数
	ClientNum int
	StartAt   int64

	OutServers map[uint64]string
}

// ChatMsg 聊消息
type ChatMsg struct {
	ID         uint64 `xorm:"pk autoincr 'id'"`
	FromDomain uint32
	ToDomain   uint32
	From       string
	Scope      uint8
	To         string
	Type       uint8  //msg type
	Text       string `xorm:"varchar(1024)"`
	Extra      string
	CreateAt   time.Time
}
