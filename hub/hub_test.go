package hub

import (
	"fmt"
	"testing"
)

func TestHub_run(t *testing.T) {
	// read config
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println(err)
		return
	}
	// new server
	hub, err := NewHub(cfg)
	if err != nil {
		fmt.Println(err)
		return
	}
	hub.run()
}
