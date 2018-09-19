package main

import (
	"RVR/algorithm"
	"flag"
	"fmt"
	"log"
)

func main() {
	fmt.Printf("version 0.3.4\n")
	var exitSignal = make(chan bool)
	mode := flag.String("mode", "node", "choose a value between node/controller")
	controlAddress := flag.String("server", "172.24.200.200:9696", "controller's address")
	flag.Parse()
	switch *mode {
	case "node":
		algorithm.StartNode(*controlAddress, exitSignal)

	case "controller":
		algorithm.StartServer(exitSignal)

	case "spawner":
		algorithm.StartSpawner(*controlAddress, exitSignal)

	default:
		log.Fatalf("Unsupported mode: %s\n Try:node/controller\n", mode)
		return
	}
	<-exitSignal
	println("exiting!")
}
