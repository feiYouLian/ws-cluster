package database

import (
	"sync"
	"time"
)

// Client 是一个具体的客户端身份对象
type Client struct {
	ID       string
	ServerID uint64
	LoginAt  uint32 //second
}

// Group 代表一个群，发往这个群的消息会转发给所有在此群中的用户。
// 聊天室不保存
type Group struct {
	rw      sync.RWMutex
	Name    string
	Clients map[string]bool
}

// Server 服务器对象
type Server struct {
	// ID 服务器Id, 自动生成，缓存到file 中，重启时 ID 不变
	ID       uint64
	BootTime time.Time
	IP       string
	Port     int
	// ClientNum 在线人数
	ClientNum int
}

// ChatMsg 聊消息
type ChatMsg struct {
	ID       uint64
	From     string
	Scope    uint8
	To       string
	Type     uint8 //msg type
	Text     string
	CreateAt time.Time
}
