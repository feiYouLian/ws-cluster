package main

// Hub 是一个中转中心，所有 clientPeer
type Hub struct {
	config      Config
	clientPeers map[int64]*ClientPeer
	serverPeer  map[string]*ServerPeer

	// Register requests from the clients.
	register chan *ClientPeer

	// Unregister requests from clients.
	unregister chan *ServerPeer
}

// NewHub 创建一个 Server 对象，并初始化
func NewHub(config *Config) *Hub {
	return nil
}
