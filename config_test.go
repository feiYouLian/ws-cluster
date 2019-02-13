package main

import (
	"reflect"
	"testing"
)

func Test_loadConfig(t *testing.T) {
	tests := []struct {
		name    string
		want    *Config
		wantErr bool
	}{
		// TODO: Add test cases.
		{"t1", &Config{
			Redis: RedisConfig{
				IP:   "192.168.0.127",
				Port: 6379,
			},
			Mysql: MysqlConfig{
				IP: "192.168.0.127",
			},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("loadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
