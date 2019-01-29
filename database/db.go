package database

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/core"
	"github.com/go-xorm/xorm"
)

// Client 是一个具体的客户端身份对象
type Client struct {
	ID       uint64
	Name     string
	ServerID string
	// Extra 额外信息
	Extra string
}

// Group 代表一个群，发往这个群的消息会转发给所有在此群中的用户。
// 聊天室不保存
type Group struct {
	ID           uint64
	Name         string
	MaxClientNum uint32
	CurClientNum uint32
	Creator      uint64
	Owner        uint64
}

// Server 服务器对象
type Server struct {
	// ID 服务器Id, 自动生成，缓存到file 中，重启时 ID 不变
	ID       string
	BootTime time.Time
	Ping     time.Time
	IP       string
	Port     int
}

// InitDb init database
func InitDb(ip, user, pwd, dbname string) *xorm.Engine {
	url := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?charset=utf8&parseTime=True&loc=Local", user, pwd, ip, dbname)
	engine, err := xorm.NewEngine("mysql", url)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// engine.ShowSQL(true)

	tbMapper := core.NewPrefixMapper(core.SnakeMapper{}, "t_")
	engine.SetTableMapper(tbMapper)

	engine.SetColumnMapper(core.SnakeMapper{})

	// err = engine.Sync2(new(UserTaskRecord))
	// if err != nil {
	// 	fmt.Println(err)
	// }
	return engine
}

// InitRedis return a redis instance
func InitRedis(ip string) *redis.Client {
	redisdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:6379", ip),
	})
	return redisdb
}
