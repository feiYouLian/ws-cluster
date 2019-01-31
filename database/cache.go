package database

// ClientCache 定义了 client 缓存操作接口
type ClientCache interface {
	AddClient(client *Client) error
	DelClient(ID uint64) (int, error)
	DelAll(ServerID string) error
	GetClient(ID uint64) (*Client, error)
}

// ServerCache 定义了服务器列表操作方法
type ServerCache interface {
	AddServer(server *Server) error
	GetServer(ID string) (*Server, error)
	DelServer(ID string) error
	GetServers() ([]Server, error)
}

// GroupCache GroupCache
type GroupCache interface {
	Join(group string, clientID uint64) error
	Leave(group string, clientID uint64) error
	GetGroupMembers(group string) ([]uint64, error)
}

// Cache 定义了缓存层接口，缓存层用于保护用户会话及服务列表等数据
type Cache struct {
	ServerCache ServerCache
	ClientCache ClientCache
}

// NewCache new Cache
func NewCache(sc ServerCache, cc ClientCache) *Cache {
	return &Cache{
		ServerCache: sc,
		ClientCache: cc,
	}
}
