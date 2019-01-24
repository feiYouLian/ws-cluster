package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"

	_ "github.com/go-sql-driver/mysql"
)

func handleInterrupt(hub *Hub, sc chan os.Signal) {

}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// read config
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println(err)
		return
	}
	// new server
	hub := NewHub(cfg)

	// listen sys.exit
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)

	go handleInterrupt(hub, sc)
}
