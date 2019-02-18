package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"

	"github.com/ws-cluster/config"
	"github.com/ws-cluster/hub"

	_ "github.com/go-sql-driver/mysql"
)

func handleInterrupt(hub *hub.Hub, sc chan os.Signal) {

	hub.Close()
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	// read config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panicln(err)
	}
	// new server
	hub, err := hub.NewHub(cfg)
	if err != nil {
		log.Panicln(err)
	}
	hub.Run()

	// listen sys.exit
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)

	go handleInterrupt(hub, sc)
}
