package database

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/go-redis/redis"
)

const (
	clientReidsPattern = "Client_%v"
	serverReidsPattern = "Server_%v"
	serversRedis       = "Server_List"
	serversDownRedis   = "Server_Down_List"
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
	cmd := c.client.Set(ckey, cli, time.Hour*6)
	_, err := cmd.Result()
	if err != nil {
		return err
	}
	return nil
}

// DelClient DelClient
func (c *RedisClientCache) DelClient(ID string) (int, error) {
	cmd := c.client.Del(fmt.Sprintf(clientReidsPattern, ID))
	aff, err := cmd.Result()
	if err != nil {
		return 0, err
	}
	return int(aff), nil
}

// GetClient GetClient
func (c *RedisClientCache) GetClient(ID string) (*Client, error) {
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

// SetServer SetServer
func (c *RedisServerCache) SetServer(server *Server) error {
	cmd := c.client.SAdd(serversRedis, server.ID)
	_, err := cmd.Result()
	if err != nil {
		return err
	}
	ser, _ := json.Marshal(server)
	skey := fmt.Sprintf(serverReidsPattern, server.ID)
	_, err = c.client.Set(skey, ser, time.Second*9).Result()
	if err != nil {
		return err
	}
	return nil
}

// GetServer GetServer
func (c *RedisServerCache) GetServer(ID uint64) (*Server, error) {
	skey := fmt.Sprintf(serverReidsPattern, ID)
	res, err := c.client.Get(skey).Result()
	if err != nil {
		return nil, err
	}
	if res == "" {
		c.client.SMove(serversRedis, serversDownRedis, ID)
		return nil, nil
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
	c.client.SMove(serversRedis, serversDownRedis, ID)
	skey := fmt.Sprintf(serverReidsPattern, ID)
	if _, err := c.client.Del(skey).Result(); err != nil {
		return err
	}
	return nil
}

// GetServers GetServers
func (c *RedisServerCache) GetServers() ([]Server, error) {
	serverIds, err := c.client.SMembers(serversRedis).Result()
	if err != nil {
		return nil, err
	}
	servers := make([]Server, 0)
	for _, serverID := range serverIds {
		ID, _ := strconv.ParseUint(serverID, 0, 64)
		server, err := c.GetServer(ID)
		if err != nil || server == nil {
			continue
		}
		servers = append(servers, *server)
	}
	return servers, nil
}

// Clean clean expired server
func (c *RedisServerCache) Clean() error {
	serverIds, err := c.client.SMembers(serversRedis).Result()
	if err != nil {
		return err
	}
	for _, serverID := range serverIds {
		if val, _ := c.client.Exists(fmt.Sprint(serverID)).Result(); val == 0 {
			c.client.SMove(serversRedis, serversDownRedis, serverID)
		}
	}
	c.client.Del(serversDownRedis)
	return nil
}

// InitRedis return a redis instance
func InitRedis(ip string, port int, pass string) (*redis.Client, error) {
	redisdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", ip, port),
		Password: pass,
	})
	_, err := redisdb.Ping().Result()
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return redisdb, nil
}
