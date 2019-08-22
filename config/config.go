package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/ws-cluster/database"

	"github.com/go-ini/ini"
)

const (
	defaultConfigName  = "conf.ini"
	defaultIDName      = "id.lock"
	defaultMessageName = "message.log"
)

var (
	configDir           = "./"
	dataDir             = "./data"
	defaultConfigFile   = filepath.Join(configDir, defaultConfigName)
	defaultIDConfigFile = filepath.Join(dataDir, defaultIDName)
)

const (
	// ModeSingle 单机启动模式
	ModeSingle = 1
	// ModeCluster 集群模式，此模式下，要求配置注册发现服务器zookeeper
	ModeCluster = 2
)

// ServerConfig ServerConfig
type ServerConfig struct {
	ID          string `description:"server logic addr"`
	Domain      int
	ListenIP    string
	ListenPort  int
	Secret      string
	Origin      string
	Mode        int
	MessageFile string
}

// RedisConfig redis config
type RedisConfig struct {
	IP       string
	Port     int
	Password string
	Db       int
}

// MysqlConfig mysql config
type MysqlConfig struct {
	IP       string
	Port     int
	User     string
	Password string
	DbName   string
}

// PeerConfig PeerConfig
type PeerConfig struct {
	MaxMessageSize int
	WriteWait      int
	PongWait       int
	PingPeriod     int
}

// Config 系统配置信息，包括 redis 配置， mongodb 配置
type Config struct {
	Server ServerConfig
	// Redis        RedisConfig
	Mysql MysqlConfig
	Peer  PeerConfig
	// Cache        Cache
	MessageStore database.MessageStore
}

// Cache 缓存服务配置
type Cache struct {
	Client database.ClientCache
	Server database.ServerCache
}

// LoadConfig LoadConfig
func LoadConfig() (*Config, error) {
	cfg, err := ini.Load(defaultConfigFile)
	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		return nil, err
	}
	var config Config
	section := cfg.Section("server")
	config.Server = ServerConfig{}
	err = section.MapTo(&config.Server)
	if err != nil {
		return nil, err
	}
	config.Server.MessageFile = filepath.Join(dataDir, defaultMessageName)

	// section = cfg.Section("redis")
	// config.Redis = RedisConfig{}
	// err = section.MapTo(&config.Redis)
	// if err != nil {
	// 	return nil, err
	// }

	section = cfg.Section("mysql")
	config.Mysql = MysqlConfig{}
	err = section.MapTo(&config.Mysql)
	if err != nil {
		return nil, err
	}
	section = cfg.Section("peer")
	config.Peer = PeerConfig{}
	err = section.MapTo(&config.Peer)
	if err != nil {
		return nil, err
	}

	// datadir
	if _, err := os.Stat(dataDir); err != nil {
		err = os.MkdirAll(dataDir, os.ModePerm)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
	}

	return &config, nil
}

// BuildServerID build a serverID
func BuildServerID() (string, error) {
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
