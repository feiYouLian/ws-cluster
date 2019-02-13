package database

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

const (
	clientReidsPattern       = "CLIENT_%d"
	serverClientReidsPattern = "SERVER_CLIENT_%d"
	serversRedis             = "SERVER_LIST"
	serverReidsPattern       = "SERVER_%d"
)

// RedisClientCache redis ClientCache
type RedisClientCache struct {
	client *redis.Client
}

// NewRedisClientCache NewRedisClientCache
func NewRedisClientCache(client *redis.Client) *RedisClientCache {
	return &RedisClientCache{client: client}
}

// AddClient AddClient
func (c *RedisClientCache) AddClient(client *Client) error {
	cli, _ := json.Marshal(client)
	ckey := fmt.Sprintf(clientReidsPattern, client.ID)
	cmd := c.client.Set(ckey, cli, time.Hour*24)
	_, err := cmd.Result()
	if err != nil {
		return err
	}
	skey := fmt.Sprintf(serverClientReidsPattern, client.ServerID)
	c.client.HSet(skey, ckey, time.Now().Unix())
	return err
}

// DelClient DelClient
func (c *RedisClientCache) DelClient(ID uint64, ServerID uint64) (int, error) {
	cmd := c.client.Del(fmt.Sprintf(clientReidsPattern, ID))
	aff, err := cmd.Result()
	if err != nil {
		return 0, err
	}
	skey := fmt.Sprintf(serverClientReidsPattern, ServerID)
	ckey := fmt.Sprintf(clientReidsPattern, ID)
	c.client.HDel(skey, ckey)
	return int(aff), nil
}

// DelAll DelAll
func (c *RedisClientCache) DelAll(ServerID uint64) error {
	skey := fmt.Sprintf(serverClientReidsPattern, ServerID)
	c.client.Del(skey)
	return nil
}

// GetClient GetClient
func (c *RedisClientCache) GetClient(ID uint64) (*Client, error) {
	ckey := fmt.Sprintf(clientReidsPattern, ID)
	cmd := c.client.Get(ckey)
	str, err := cmd.Result()
	if err != nil {
		return nil, err
	}
	client := &Client{}
	err = json.Unmarshal([]byte(str), client)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// RedisServerCache RedisServerCache
type RedisServerCache struct {
	client *redis.Client
}

// NewRedisServerCache NewRedisServerCache
func NewRedisServerCache(client *redis.Client) *RedisServerCache {
	return &RedisServerCache{client: client}
}

// AddServer AddServer
func (c *RedisServerCache) AddServer(server *Server) error {
	ser, _ := json.Marshal(server)
	skey := fmt.Sprintf(serverReidsPattern, server.ID)
	cmd := c.client.HSet(serversRedis, skey, ser)
	_, err := cmd.Result()
	if err != nil {
		return err
	}
	return nil
}

// GetServer GetServer
func (c *RedisServerCache) GetServer(ID uint64) (*Server, error) {
	skey := fmt.Sprintf(serverReidsPattern, ID)
	cmd := c.client.HGet(serversRedis, skey)
	res, err := cmd.Result()
	if err != nil {
		return nil, err
	}
	server := &Server{}
	err = json.Unmarshal([]byte(res), server)
	if err != nil {
		return nil, err
	}
	return server, nil
}

// DelServer DelServer
func (c *RedisServerCache) DelServer(ID uint64) error {
	skey := fmt.Sprintf(serverReidsPattern, ID)
	cmd := c.client.HDel(serversRedis, skey)
	_, err := cmd.Result()
	if err != nil {
		return err
	}
	return nil
}

// GetServers GetServers
func (c *RedisServerCache) GetServers() ([]Server, error) {
	cmd := c.client.HGetAll(serversRedis)
	res, err := cmd.Result()
	if err != nil {
		return nil, err
	}
	servers := make([]Server, len(res))
	for _, item := range res {
		server := Server{}
		err := json.Unmarshal([]byte(item), &server)
		if err != nil {
			continue
		}
		servers = append(servers, server)
	}
	return servers, nil
}

// RedisGroupCache RedisGroupCache
type RedisGroupCache struct {
	client *redis.Client
}

// InitRedis return a redis instance
func InitRedis(ip string, port int, pass string) *redis.Client {
	redisdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", ip, port),
		Password: pass,
	})
	return redisdb
}
