package main

import (
	"RVR/message"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"testing"
	"time"
)

var testSetupParams = ProtocolRPCSetupParams{
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

func Test_getLocalAddress(t *testing.T){
	// https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
	ifaces, _ := net.Interfaces()
	// handle err
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		// handle err
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				println("this is IPNet")
				ip = v.IP
			case *net.IPAddr:
				println("this is IPAddr")
				ip = v.IP
			}
			// process IP address
			fmt.Printf("The ip address is: %s\n", ip.String())
		}
	}

	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	fmt.Printf("The outbound address is %s\n", localAddr.IP.String())
}

func Test_Controller(t *testing.T) {
	TEST_SIZE := 6
	viewDist := make(map[uint64]float64)
	// prepare the View distribution
	for i := 0; i < 10; i++ {
		viewDist[uint64(i)] = float64(i) / 10
	}

	// start the controller
	controller := ControllerState{testSetupParams, make([]message.Identity, 0), ""}
	controller.startListen()
	CONTROL_ADDRESS = controller.address

	// Dial the controller
	client, _ := rpc.Dial("tcp", CONTROL_ADDRESS)

	// start the nodes
	syncLock := make(chan bool)
	peers := make([] ProtocolState, TEST_SIZE)
	for i, _ := range peers {
		go func(i int) {
			peers[i].getReady()
			syncLock <- true
		}(i)
	}
	for i := 0; i < TEST_SIZE; i++ {
		<-syncLock // wait for all peers gets ready
	}
	time.Sleep(100 * time.Millisecond) // time needed for all the registration messages to get through
	// check the controller state
	fmt.Printf("Controller received %d registrations.\n", len(controller.peerList))

	// controller setup the protocol
	client.Go("ControllerState.SetupProtocol",1, nil, nil)
	time.Sleep(500 * time.Millisecond)

	// controller start the protocol
	client.Go("ControllerState.StartProtocol",1, nil, nil)

	// wait until Finished
	for flag:=true; flag; {
		flag = false
		time.Sleep(1 * time.Second)
		for i, _ := range peers {
			if !peers[i].Finished {
				flag = true
				break;
			}
		}
	}

}
