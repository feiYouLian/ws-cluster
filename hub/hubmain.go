package hub

import (
	"log"
	"os"
	"os/signal"
	"runtime"

	"github.com/ws-cluster/database"
)

func handleInterrupt(hub *Hub, sc chan os.Signal) {
	select {
	case <-sc:
		hub.Close()
	}
}

// RunMain RunMain
func RunMain() {

	runtime.GOMAXPROCS(runtime.NumCPU())
	// read config
	conf, err := LoadConfig()
	if err != nil {
		log.Panicln(err)
	}

	// build a client instance of redis
	if conf.dc != nil {
		engine := database.InitMysqlDb(conf.dc.DbSource)
		conf.ms = database.NewDbMessageStore(engine)
	}

	// var cache config.Cache

	// if conf.Server.Mode == config.ModeCluster {
	// 	redis, err := database.InitRedis(conf.Redis.IP, conf.Redis.Port, conf.Redis.Password, conf.Redis.Db)
	// 	if err != nil {
	// 		log.Panicln(err)
	// 	}
	// 	t1 := time.Now()
	// 	serverTime, err := redis.Time().Result()
	// 	t2 := time.Now()
	// 	if err != nil {
	// 		log.Panicln(err)
	// 	}
	// 	log.Println("redis time", serverTime)
	// 	serverTime = serverTime.Add(t2.Sub(t1))

	// 	if math.Abs(float64(serverTime.Sub(time.Now())/time.Millisecond)) > 500 {
	// 		log.Panicln("system time is incorrect", time.Now())
	// 	}
	// 	cache.Client = database.NewRedisClientCache(redis)
	// 	cache.Server = database.NewRedisServerCache(redis)
	// }
	// conf.Cache = cache

	// new server
	hub, err := NewHub(conf)
	if err != nil {
		log.Panicln(err)
	}
	// listen sys.exit
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)

	go handleInterrupt(hub, sc)

	hub.Run()
}
