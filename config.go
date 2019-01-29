package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-ini/ini"
)

const (
	defaultConfigName = "conf.ini"
)

var (
	defaultDir        = "./"
	defaultConfigFile = filepath.Join(defaultDir, defaultConfigName)
)

// Config 系统配置信息，包括 redis 配置， mongodb 配置
type Config struct {
	ServerID     string `description:"server id"`
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
	if !section.HasKey("id") {
		section.Key("id").SetValue(fmt.Sprintf("S%d", time.Now().UnixNano()))
		change = true
	}
	config.ServerID = section.Key("id").String()
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
	return &config, nil
}
