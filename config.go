package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-ini/ini"
)

const (
	defaultConfigName = "conf.ini"
	defaultIDName     = "id.lock"
)

var (
	defaultDir          = "./"
	defaultConfigFile   = filepath.Join(defaultDir, defaultConfigName)
	defaultServerIDFile = filepath.Join(defaultDir, defaultIDName)
)

// Config 系统配置信息，包括 redis 配置， mongodb 配置
type Config struct {
	ServerID  string `description:"server id"`
	RedisIP   string `description:"redis ip"`
	RedisPort int
	MysqlIP   string
	MysqlPort int
}

func loadConfig() (*Config, error) {
	cfg, err := ini.Load(defaultConfigFile)
	if err != nil {
		fmt.Printf("Fail to read file: %v", err)
		return nil, err
	}
	var config Config
	section := cfg.Section("server")
	if !section.HasKey("id") {
		section.Key("id").SetValue(fmt.Sprintf("S%d", time.Now().UnixNano()))
		if err := cfg.SaveTo(defaultConfigFile); err != nil {
			return nil, err
		}
	}
	config.ServerID = section.Key("id").String()

	section = cfg.Section("redis")
	config.RedisIP = section.Key("ip").String()
	config.RedisPort, err = section.Key("port").Int()
	if err != nil {
		return nil, err
	}

	section = cfg.Section("mysql")
	config.MysqlIP = section.Key("ip").String()

	return &config, nil
}
