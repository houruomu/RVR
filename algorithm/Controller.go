package algorithm

import (
	"RVR/message"
	"bufio"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"strings"
	"time"
)

// this packet is to monitor and coordinate the nodes

var DefaultSetupParams = ProtocolRPCSetupParams{
	20 * time.Millisecond,
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
	ExitSignal  chan bool
	SetupParams ProtocolRPCSetupParams
	PeerList    []message.Identity
	Address     string
	ServerList  []string
}
func (c *ControllerState) checkConnection(){
	connectedPeers := make([]message.Identity, 0)
	for i, _ := range c.PeerList{
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			log.Print("Peer %s disconnected.\n", c.PeerList[i].Address)
			continue
		}
		connectedPeers = append(connectedPeers, c.PeerList[i])
		client.Close()
	}
	c.PeerList = connectedPeers

	connectedServers := make([]string, 0)
	for i, _ := range c.ServerList{
		client, err := rpc.Dial("tcp", c.ServerList[i])
		if err != nil {
			log.Print("Peer %s disconnected.\n", c.ServerList[i])
			continue
		}
		connectedServers = append(connectedServers, c.ServerList[i])
		client.Close()
	}
	c.ServerList = connectedServers
}

func (c *ControllerState) spawnEvenly(count int){
	fmt.Printf("Spawning Evenly %d instances\n", count)
	c.checkConnection()
	for i, _ := range c.ServerList{
		numInstance := count / len(c.ServerList)
		if numInstance < count % len(c.ServerList){
			numInstance++
		}
		c.Spawn(c.ServerList[i], numInstance)
	}
}

func (c *ControllerState) Spawn(addr string, count int) {
	fmt.Printf("Spawning %d instances at %s\n", count, addr)
	c.ServerList = append(c.ServerList, addr)
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		log.Print("Spawning:", err.Error())
	}
	defer client.Close()
	client.Call("SpawnerState.Spawn", count, nil)
}

func (c *ControllerState) RegisterServer(addr string, rtv *int) error {
	c.ServerList = append(c.ServerList, addr)
	fmt.Printf("New Server registered at controller: %s\n", addr)
	return nil
}

func (c *ControllerState) Register(id message.Identity, rtv *int) error {
	c.PeerList = append(c.PeerList, id)
	fmt.Printf("New Peer registered at controller: %s\n", id.Address)
	return nil
}

func (c *ControllerState) setupRandomizedView() error {
	for i, _ := range c.PeerList {
		view := make([]uint64, 0)
		for j, _ := range c.PeerList {
			if (rand.Float32() < 0.4) {
				view = append(view, c.PeerList[j].GetUUID())
			}
		}
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			log.Print("Setting view:", err.Error())
			return nil
		}
		defer client.Close()
		client.Go("ProtocolState.SetView", view, nil, nil)
	}
	return nil
}

func (c *ControllerState) SetupProtocol(ph1 int, ph2 *int) error {
	c.SetupParams.InitView = c.PeerList
	nEstimate := float64(len(c.PeerList))
	c.SetupParams.X = int(math.Ceil(math.Log(nEstimate)/math.Log(math.Log(nEstimate))+4.0))*c.SetupParams.L + c.SetupParams.Offset
	for i, _ := range c.PeerList {
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			log.Print("Setup protocol:", err.Error())
			return nil
		}
		defer client.Close()
		client.Go("ProtocolState.Setup", c.SetupParams, nil, nil)
	}
	c.setupRandomizedView()
	print(c.SetupParams.String())
	return nil
}

func (c *ControllerState) KillNodes(ph1 int, ph2 *int) error {
	for i, _ := range c.PeerList {
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			log.Print("Killing peer:", err.Error())
			return nil
		}
		defer client.Close()
		client.Call("ProtocolState.Exit", 1, nil)
	}
	return nil
}

func (c *ControllerState) KillServers(ph1 int, ph2 *int) error {
	for i, _ := range c.ServerList {
		client, err := rpc.Dial("tcp", c.ServerList[i])
		if err != nil {
			log.Print("Killing server:", err.Error())
			return nil
		}
		defer client.Close()
		client.Call("SpawnerState.Exit", 1, nil)
	}
	return nil
}

func (c *ControllerState) StartProtocol(ph1 int, ph2 *int) error {
	for i, _ := range c.PeerList {
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			log.Print("Starting Protocol:", err.Error())
			return nil
		}
		defer client.Close()
		client.Go("ProtocolState.Start", 1, nil, nil)
	}
	return nil
}

func (c *ControllerState) checkState(address string) string {
	state := ProtocolState{}
	client, err := rpc.Dial("tcp", address)
	if err != nil {
		log.Print("Check State:", err.Error())
		return "Error\n"
	}
	defer client.Close()
	client.Call("ProtocolState.RetrieveState", 1, &state)
	return state.String()
}

func (c *ControllerState) report() string {
	state := make([]ProtocolState, 0)
	for i,_ := range c.PeerList{
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			log.Print("Check State:", err.Error())
			continue
		}
		defer client.Close()
		newState := ProtocolState{}
		client.Call("ProtocolState.RetrieveState", 1, &newState)
		state = append(state, newState)
	}
	analysis := Data{state, c.SetupParams}
	return analysis.Report()
}

func (c *ControllerState) StartListen() {
	c.PeerList = make([]message.Identity, 0)
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
	myPort := l.Addr().String()[strings.LastIndex(l.Addr().String(), ":"):]
	c.Address = myIP + myPort
	fmt.Printf("Controller started at Address: %s\n", c.Address)

	for {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := scanner.Text()
			switch text {
			case "auto":
				c.autoTest(10, DefaultSetupParams)
			case "setup":
				c.SetupProtocol(1, nil)
			case "start":
				c.StartProtocol(1, nil)
			case "state":
				// pick a random node to retrieve state
				nodeAddr := c.PeerList[rand.Int()%len(c.PeerList)].Address
				print(c.checkState(nodeAddr))
			case "report":
				// pick a random node to retrieve state
				print(c.report())
			case "reset":
				c.KillNodes(1, nil)
				c.PeerList = make([]message.Identity, 0)
			case "spawn":
				serverAddr := c.ServerList[rand.Int()%len(c.ServerList)]
				c.Spawn(serverAddr, 1)
			case "exit":
				c.KillNodes(1, nil)
				c.KillServers(1, nil)
				c.ExitSignal <- true
			default:
				fmt.Printf("try setup/start instead of %s\n", text)
			}
		}
	}

}

func (c *ControllerState) autoTest(size int, params ProtocolRPCSetupParams){
	c.KillNodes(1, nil)
	c.PeerList = make([]message.Identity, 0)

	for len(c.PeerList) < size{
		fmt.Printf("We have %d peers now\n", len(c.PeerList))
		c.spawnEvenly(size - len(c.PeerList))
		time.Sleep(10 * time.Second)
	}

	c.SetupParams = params
	c.SetupProtocol(1, nil)
	c.StartProtocol(1, nil)

	print("Auto-test starting!\n")
	var report string
	for{
		time.Sleep(10 * time.Second)
		report = c.report()
		if report[0] == 't'{
			// finished
			break
		}
		print("protocol haven't finished!\n")
	}
	print("Auto-test completed!\n")
	fmt.Printf("%d, %d, %s", size, len(c.PeerList), report)
}



func StartServer(exitSignal chan bool){
	c := ControllerState{exitSignal, DefaultSetupParams, make([]message.Identity, 0), "", make([]string, 0)}
	go c.StartListen()
}
