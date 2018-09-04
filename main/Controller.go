package main

import (
	"RVR/message"
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"strings"
	"time"
)

// this packet is to monitor and coordinate the nodes

var defaultSetupParams = ProtocolRPCSetupParams{
	100 * time.Millisecond,
	2,
	0.01,
	0.01,
	2,
	2,
	0.01,
	message.Identity{},
	nil,
}

type ControllerState struct {
	setupParams ProtocolRPCSetupParams
	peerList    []message.Identity
	address     string
}


func (c *ControllerState) Register(id message.Identity, rtv *int) error {
	c.peerList = append(c.peerList, id)
	fmt.Printf("New Peer registered at controller: %s\n", id.Address)
	return nil
}

func (c *ControllerState) SetupRandomizedView(ph1 int, ph2 *int) error {
	for i, _ := range c.peerList {
		view := make([]uint64, 0)
		for j, _ := range c.peerList{
			if (rand.Float32() < 0.4){
				view = append(view, c.peerList[j].GetUUID())
			}
		}
		client, err := rpc.Dial("tcp", c.peerList[i].Address)
		if err != nil {
			log.Fatal("dialing:", err.Error())
			return nil
		}
		defer client.Close()
		client.Go("ProtocolState.SetView", view, nil, nil)
	}
	return nil
}

func (c *ControllerState) SetupProtocol(ph1 int, ph2 *int) error {
	c.setupParams.InitView = c.peerList
	for i, _ := range c.peerList {
		client, err := rpc.Dial("tcp", c.peerList[i].Address)
		if err != nil {
			log.Fatal("dialing:", err.Error())
			return nil
		}
		defer client.Close()
		client.Go("ProtocolState.Setup", c.setupParams, nil, nil)
	}
	c.SetupRandomizedView(1, nil)
	return nil
}

func (c *ControllerState) StartProtocol(ph1 int, ph2 *int) error {
	for i, _ := range c.peerList {
		client, err := rpc.Dial("tcp", c.peerList[i].Address)
		if err != nil {
			log.Fatal("dialing:", err.Error())
			return nil
		}
		defer client.Close()
		client.Go("ProtocolState.Start", 1, nil, nil)
	}
	return nil
}

func (c *ControllerState) startListen() {
	c.peerList = make([]message.Identity, 0)
	handler := rpc.NewServer() // allows multiple rpc at a time
	handler.Register(c)
	l, e := net.Listen("tcp", ":9696") // Listen on Specific port
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
	myPort := l.Addr().String()[strings.LastIndex(l.Addr().String(),":"):]
	c.address = myIP + myPort
	fmt.Printf("Controller started at address: %s\n", c.address)

	for{
		reader := bufio.NewReader(os.Stdin)
		text, _ := reader.ReadString('\n')
		switch text {
		case "setup\n":
			c.SetupProtocol(1, nil)
		case "start\n":
			c.StartProtocol(1, nil)
		default:
			fmt.Printf("try setup/start instead of %s \n", text)
		}
	}

}
