package algorithm

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
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
	handler := rpc.NewServer() // allows multiple rpc at a time
	handler.Register(s)
	l, e := net.Listen("tcp", ":0") // Listen on OS chosen addr
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Print("Error: accept rpc connection", err.Error())
				continue
			}
			go handler.ServeConn(conn)
		}
	}()
	myIP := GetOutboundAddr()
	myPort := l.Addr().String()[strings.LastIndex(l.Addr().String(), ":"):]
	addr := myIP + myPort
	// report to controller
	client, err := rpc.Dial("tcp", s.ControlAddress)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer client.Close()
	fmt.Printf("Spawner Ready: %s\n", addr)
	client.Go("ControllerState.RegisterServer", addr, nil, nil)
}

func StartSpawner(controlAddr string, exitSignal chan bool){
	server := SpawnerState{ControlAddress:controlAddr, ExitSignal:exitSignal}
	server.Start()
}