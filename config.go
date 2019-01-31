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

// Config 系统配置信息，包括 redis 配置， mongodb 配置
type Config struct {
	ServerID     uint64 `description:"server id"`
	ServerAddr   string
	ServerListen int
	ServerSecret string
	RedisIP      string `description:"redis ip"`
	RedisPort    int
	MysqlIP      string
	MysqlPort    int
}

func loadConfig() (*Config, error) {
	cfg, err := ini.Load(defaultConfigFile)
	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		return nil, err
	}
	var config Config
	var change = false
	section := cfg.Section("server")

	config.ServerAddr = section.Key("addr").String()
	config.ServerSecret = section.Key("secret").String()
	config.ServerListen = section.Key("listen").MustInt(6379)

	section = cfg.Section("redis")
	config.RedisIP = section.Key("ip").String()
	config.RedisPort = section.Key("port").MustInt(6379)

	section = cfg.Section("mysql")
	config.MysqlIP = section.Key("ip").String()
	config.MysqlPort = section.Key("port").MustInt(3306)

	if change {
		if err := cfg.SaveTo(defaultConfigFile); err != nil {
			return nil, err
		}
	}

	_, err = os.Stat(defaultIDConfigFile)
	if err != nil {
		sid := fmt.Sprintf("%d", time.Now().UnixNano())
		ioutil.WriteFile(defaultIDConfigFile, []byte(sid), os.ModeType)
	}
	fb, err := ioutil.ReadFile(defaultIDConfigFile)
	if err != nil {
		return nil, err
	}
	config.ServerID, _ = strconv.ParseUint(string(fb), 0, 64)

	return &config, nil
}
