package main

import (
	"log"
	"math"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/ws-cluster/config"
	"github.com/ws-cluster/database"
	"github.com/ws-cluster/hub"

	_ "github.com/go-sql-driver/mysql"
)

func handleInterrupt(hub *hub.Hub, sc chan os.Signal) {
	select {
	case <-sc:
		hub.Close()
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// read config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicln(err)
	}

	// build a client instance of redis
	mysqldb := database.InitDb(cfg.Mysql.IP, cfg.Mysql.Port, cfg.Mysql.User, cfg.Mysql.Password, cfg.Mysql.DbName)
	cfg.MessageStore = database.NewMysqlMessageStore(mysqldb)

	var cache config.Cache
	cache.Group = database.NewMemGroupCache()
	if cfg.Server.Mode == config.ModeCluster {
		redis := database.InitRedis(cfg.Redis.IP, cfg.Redis.Port, cfg.Redis.Password)

		t1 := time.Now()
		serverTime, err := redis.Time().Result()
		t2 := time.Now()
		if err != nil {
			log.Panicln(err)
		}
		serverTime = serverTime.Add(t2.Sub(t1))

		if math.Abs(float64(serverTime.Sub(time.Now())/time.Millisecond)) > 500 {
			log.Panicln("system time is incorrect")
		}
		cache.Client = database.NewRedisClientCache(redis)
		cache.Server = database.NewRedisServerCache(redis)
	}
	cfg.Cache = cache

	// new server
	hub, err := hub.NewHub(cfg)
	if err != nil {
		log.Panicln(err)
	}
	// listen sys.exit
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)

	go handleInterrupt(hub, sc)

	hub.Run()
}
