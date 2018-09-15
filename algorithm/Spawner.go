package algorithm

import (
	"fmt"
	"strings"
	"time"
)

type SpawnerState struct{
	ControlAddress string
	ExitSignal chan bool
}

func (s *SpawnerState) Spawn(count int, rtv *int) error{
	for i := 0; i < count ; i++  {
		go func(){
			_exitSignal := make(chan bool)
			StartNode(s.ControlAddress, _exitSignal)
			<- _exitSignal
			print("node exiting\n")
		}()
	}
	return nil
}

func (s *SpawnerState) Exit(count int, rtv *int) error{
	go func(term chan bool){time.Sleep(100 * time.Microsecond); term <- true}(s.ExitSignal)
	return nil
}

func (s *SpawnerState) Start() {
	myIP := GetOutboundAddr()
	portString := ListenRPC(":0", s)
	myPort := portString[strings.LastIndex(portString, ":"):]
	addr := myIP + myPort
	// report to controller

	RpcCall(s.ControlAddress, "ControllerState.RegisterServer", addr, nil)
	fmt.Printf("Spawner Ready: %s\n", addr)

}

func StartSpawner(controlAddr string, exitSignal chan bool){
	server := SpawnerState{ControlAddress:controlAddr, ExitSignal:exitSignal}
	server.Start()
}

func (s *SpawnerState) BlackHole(msg []byte, rtv *int) error {
	// this is a blackhole function for measuring ping value
	for i, _ := range msg {
		if msg[i] == 0 {
			return nil
		}
	}
	return nil
}
