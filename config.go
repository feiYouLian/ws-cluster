package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-ini/ini"
)

const (
	defaultConfigName = "conf.ini"
	defaultIDName     = ".lock"
)

var (
	defaultDir          = "./"
	defaultConfigFile   = filepath.Join(defaultDir, defaultConfigName)
	defaultIDConfigFile = filepath.Join(defaultDir, defaultIDName)
)

// ServerConfig ServerConfig
type ServerConfig struct {
	ID     uint64 `description:"server id"`
	Addr   string
	Listen int
	Secret string
}

// RedisConfig redis config
type RedisConfig struct {
	IP       string
	Port     int
	Password string
}

// MysqlConfig mysql config
type MysqlConfig struct {
	IP       string
	Port     int
	User     string
	Password string
	DbName   string
}

// MessageConfig message config
type MessageConfig struct {
	QueueLen int
}

// Config 系统配置信息，包括 redis 配置， mongodb 配置
type Config struct {
	Server  ServerConfig
	Redis   RedisConfig
	Mysql   MysqlConfig
	Message MessageConfig
}

func loadConfig() (*Config, error) {
	cfg, err := ini.Load(defaultConfigFile)
	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		return nil, err
	}
	var config Config
	section := cfg.Section("server")
	var serverConfig ServerConfig
	err = section.MapTo(&serverConfig)
	if err != nil {
		return nil, err
	}

	section = cfg.Section("redis")
	var redisConfig RedisConfig
	err = section.MapTo(&redisConfig)
	if err != nil {
		return nil, err
	}

	section = cfg.Section("mysql")
	var mysqlConfig MysqlConfig
	err = section.MapTo(&mysqlConfig)
	if err != nil {
		return nil, err
	}

	section = cfg.Section("message")
	var messageConfig MessageConfig
	err = section.MapTo(&messageConfig)
	if err != nil {
		return nil, err
	}

	// deal server id
	_, err = os.Stat(defaultIDConfigFile)
	if err != nil {
		sid := fmt.Sprintf("%d", time.Now().UnixNano())
		ioutil.WriteFile(defaultIDConfigFile, []byte(sid), os.ModeType)
	}
	fb, err := ioutil.ReadFile(defaultIDConfigFile)
	if err != nil {
		return nil, err
	}
	config.Server.ID, _ = strconv.ParseUint(string(fb), 0, 64)

	return &config, nil
}
