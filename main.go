package main

import (
	_ "net/http/pprof"

	"github.com/ws-cluster/hub"
)

func main() {
	hub.Main()
}
