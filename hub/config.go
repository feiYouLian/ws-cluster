package hub

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/ws-cluster/database"
)

const (
	// defaultConfigName  = "conf.ini"
	defaultIDName      = "id.lock"
	defaultMessageName = "message.log"
)

const (
	// Time allowed to write a message to the peer.
	defaultWriteWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	defaultPongWait = 20 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	defaultPingPeriod = 10 * time.Second

	// Maximum message size allowed from peer.
	defaultMaxMessageSize = 512
)

var (
	// configDir = "./"
	defaultDataDir  = "./data"
	defaultDbDriver = "mysql"
	// defaultConfigFile   = filepath.Join(configDir, defaultConfigName)
)

type serverConfig struct {
	ID                 string `description:"server logic addr"`
	AcceptDomains      []int
	ListenHost         string
	AdvertiseClientURL *url.URL
	AdvertiseServerURL *url.URL
	Secret             string
	Origins            string
	MessageFile        string
}

type peerConfig struct {
	MaxMessageSize int
	WriteWait      time.Duration
	PongWait       time.Duration
	PingPeriod     time.Duration
}

type databaseConfig struct {
	DbDriver string
	DbSource string
}

// Config 系统配置信息，包括 redis 配置， mongodb 配置
type Config struct {
	// server
	sc serverConfig
	dc *databaseConfig
	//client peer config
	cpc     peerConfig
	dataDir string
	// Cache        Cache
	ms database.MessageStore
}

// LoadConfig LoadConfig
func LoadConfig() (*Config, error) {
	// cfg, err := ini.Load(defaultConfigFile)
	// if err != nil {
	// 	fmt.Printf("Fail to read file: %v", err)
	// 	return nil, err
	// }
	var conf Config
	// section := cfg.Section("server")

	conf.sc = serverConfig{}

	flag.StringVar(&conf.sc.ListenHost, "listen-host", "0.0.0.0:8380", "listen host,format ip:port")

	clientURL := flag.String("advertise-client-url", "", "the url is to listen on for client traffic")
	u, err := url.Parse(*clientURL)
	if err != nil {
		return nil, err
	}
	conf.sc.AdvertiseClientURL = u

	serverURL := flag.String("advertise-server-url", "", "use for server connecting")
	u, err = url.Parse(*serverURL)
	if err != nil {
		return nil, err
	}
	conf.sc.AdvertiseClientURL = u

	conf.cpc = peerConfig{}
	flag.IntVar(&conf.cpc.MaxMessageSize, "client-max-msg-size", defaultMaxMessageSize, "Maximum message size allowed from client.")
	flag.DurationVar(&conf.cpc.WriteWait, "client-write-wait", defaultWriteWait, "Time allowed to write a message to the client")
	flag.DurationVar(&conf.cpc.PingPeriod, "client-ping-period", defaultWriteWait, "Send pings to client with this period. Must be less than pongWait")
	flag.DurationVar(&conf.cpc.PongWait, "client-pong-wait", defaultWriteWait, "Time allowed to read the next pong message from the client")

	dbsource := *flag.String("db-source", "", "database source, just support mysql")
	if dbsource != "" {
		conf.dc = new(databaseConfig)
		flag.StringVar(&conf.dc.DbDriver, "db-driver", defaultDbDriver, "database dirver, just support mysql")
	}
	// err = section.MapTo(&config.Server)
	// if err != nil {
	// 	return nil, err
	// }
	// config.Server.MessageFile = filepath.Join(dataDir, defaultMessageName)

	// section = cfg.Section("mysql")
	// config.Mysql = MysqlConfig{}
	// err = section.MapTo(&config.Mysql)
	// if err != nil {
	// 	return nil, err
	// }
	// section = cfg.Section("peer")
	// config.Peer = PeerConfig{}
	// err = section.MapTo(&config.Peer)
	// if err != nil {
	// 	return nil, err
	// }

	// datadir
	conf.dataDir = *flag.String("data-dir", defaultDataDir, "data directory")

	flag.Parse()

	conf.sc.MessageFile = filepath.Join(conf.dataDir, defaultMessageName)
	if _, err := os.Stat(conf.dataDir); err != nil {
		err = os.MkdirAll(conf.dataDir, os.ModePerm)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
	}

	conf.sc.ID, err = BuildServerID(conf.dataDir)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}

// BuildServerID build a serverID
func BuildServerID(dataDir string) (string, error) {
	defaultIDConfigFile := filepath.Join(dataDir, defaultIDName)
	// deal server id
	_, err := os.Stat(defaultIDConfigFile)
	if err != nil {
		sid := fmt.Sprintf("%d", time.Now().Unix())
		ioutil.WriteFile(defaultIDConfigFile, []byte(sid), 0644)
	}
	fb, err := ioutil.ReadFile(defaultIDConfigFile)
	if err != nil {
		return "", err
	}
	return string(fb), nil
}
