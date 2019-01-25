package database

import "github.com/go-redis/redis"

// RedisClientCache redis ClientCache
type RedisClientCache struct {
	client *redis.Client
}

// AddClient AddClient
func (c *RedisClientCache) AddClient(client *Client) error {
	return nil
}

// DelClient DelClient
func (c *RedisClientCache) DelClient(ID int64) (int, error) {
	return 0, nil
}

// DelAll DelAll
func (c *RedisClientCache) DelAll(ServerID string) error {
	return nil
}

// GetClient GetClient
func (c *RedisClientCache) GetClient(ID int64) (*Client, error) {
	return nil, nil
}

// RedisServerCache RedisServerCache
type RedisServerCache struct {
	client *redis.Client
}

// AddServer AddServer
func (c *RedisServerCache) AddServer(server *Server) error {
	return nil
}

// GetServer GetServer
func (c *RedisServerCache) GetServer(ID string) (*Server, error) {
	return nil, nil
}

// DelServer DelServer
func (c *RedisServerCache) DelServer(ID string) error {
	return nil
}

// GetServers GetServers
func (c *RedisServerCache) GetServers() ([]Server, error) {
	return nil, nil
}
