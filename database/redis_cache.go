package database

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis"
)

const (
	clientReidsPattern = "Client_%v"
	serversRedis       = "Server_List"
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
	ser, _ := json.Marshal(server)
	cmd := c.client.HSet(serversRedis, fmt.Sprint(server.ID), ser)
	_, err := cmd.Result()
	if err != nil {
		return err
	}
	return nil
}

// GetServer GetServer
func (c *RedisServerCache) GetServer(ID uint64) (*Server, error) {
	cmd := c.client.HGet(serversRedis, fmt.Sprint(ID))
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
	cmd := c.client.HDel(serversRedis, fmt.Sprint(ID))
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
	i := 0
	for _, item := range res {
		server := Server{}
		err := json.Unmarshal([]byte(item), &server)
		if err != nil {
			continue
		}
		servers[i] = server
		i++
	}
	return servers, nil
}

// InitRedis return a redis instance
func InitRedis(ip string, port int, pass string) *redis.Client {
	redisdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", ip, port),
		Password: pass,
	})
	_, err := redisdb.Ping().Result()
	if err != nil {
		log.Println(err)
	}
	return redisdb
}
