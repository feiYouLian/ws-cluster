package database

import (
	"fmt"
	"testing"
)

func TestInitRedis(t *testing.T) {
	redis, _ := InitRedis("192.168.0.127", 6379, "123456")
	keys, _ := redis.Keys("*").Result()
	fmt.Println(keys)

	// redis.Del(keys...)
}
