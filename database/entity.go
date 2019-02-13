package database

import (
	"time"
)

// Client 是一个具体的客户端身份对象
type Client struct {
	ID       uint64
	Name     string
	ServerID uint64
	// Extra 额外信息
	Extra string
}

// Group 代表一个群，发往这个群的消息会转发给所有在此群中的用户。
// 聊天室不保存
type Group struct {
	Name    string
	Clients map[uint64]bool
}

// Server 服务器对象
type Server struct {
	// ID 服务器Id, 自动生成，缓存到file 中，重启时 ID 不变
	ID       uint64
	BootTime time.Time
	Ping     time.Time
	IP       string
	Port     int
}

// ChatMsg 单聊消息
type ChatMsg struct {
	ID       uint64
	From     uint64
	To       uint64
	Type     uint8 //msg type
	Text     string
	CreateAt time.Time
}
