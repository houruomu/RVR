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
	ExitSignal  chan bool
	SetupParams ProtocolRPCSetupParams
	PeerList    []message.Identity
	Address     string
	ServerList  []string
	NetworkMetric []PingValueReport
}
func (c *ControllerState) checkConnection(){
	connectedPeers := make([]message.Identity, 0)
	for i, _ := range c.PeerList{
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			log.Printf("Peer %s disconnected.\n", c.PeerList[i].Address)
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
			log.Printf("Server %s disconnected.\n", c.ServerList[i])
			continue
		}
		connectedServers = append(connectedServers, c.ServerList[i])
		client.Close()
	}
	c.ServerList = connectedServers
}

func (c *ControllerState) spawnEvenly(count int){
	c.checkConnection()
	for i, _ := range c.ServerList{
		numInstance := count / len(c.ServerList)
		if i < count % len(c.ServerList){
			numInstance++
		}
		c.Spawn(c.ServerList[i], numInstance)
	}
}

func (c *ControllerState) Spawn(addr string, count int) {
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
			if (rand.Float32() < 0.7) {
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

	connectedPeers := make([]message.Identity, 0)
	for i, _ := range c.PeerList {
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			log.Print("Setup protocol:", err.Error())
			return nil
		}
		defer client.Close()
		err = client.Call("ProtocolState.Setup", c.SetupParams, nil)
		for j := 0; j < 5 && err != nil; j++{
			time.Sleep(100 * time.Millisecond)
			err = client.Call("ProtocolState.Setup", c.SetupParams, nil)
		}
		if err != nil{
			client.Call("ProtocolState.Exit", 1, nil)
		}else{
			connectedPeers = append(connectedPeers, c.PeerList[i])
		}
	}
	c.PeerList = connectedPeers
	c.setupRandomizedView()
	print(c.SetupParams.String())
	return nil
}

func (c *ControllerState) killNode(addr string){
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		log.Print("Killing peer:", err.Error())
	}
	defer client.Close()
	client.Call("ProtocolState.Exit", 1, nil)
}

func (c *ControllerState) KillNodes(ph1 int, ph2 *int) error {
	for i, _ := range c.PeerList {
		c.killNode(c.PeerList[i].Address)
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
	callList := make([]*rpc.Call, len(c.PeerList))
	for i, _ := range c.PeerList {
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			log.Print("Starting Protocol:", err.Error())
			return nil
		}
		defer client.Close()
		callList[i] = client.Go("ProtocolState.Start", 1, nil, nil)
	}
	time.Sleep(time.Duration(c.SetupParams.Offset + 1) * c.SetupParams.RoundDuration)

	startedPeers := make([]message.Identity, 0)
	for i, call := range callList{
		select{
		case <- call.Done:
			startedPeers = append(startedPeers, c.PeerList[i])
		default:
			c.killNode(c.PeerList[i].Address)
		}
	}
	c.PeerList = startedPeers
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

func (c *ControllerState) AcceptReport(report PingValueReport, ph2 *int) error{
	c.NetworkMetric = append(c.NetworkMetric, report)
	return nil
}

func (c *ControllerState) measure() {
	c.NetworkMetric = make([]PingValueReport, 0)
	for i, _ := range c.PeerList {
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			continue
		}
		defer client.Close()
		client.Go("ProtocolState.PingReport", 2000, nil, nil)
	}

	time.Sleep(6 * c.SetupParams.RoundDuration)

	// process
	peerCount := len(c.PeerList)
	reportCount := len(c.NetworkMetric)
	connectionCount := 0
	failCount := 0
	totalDelay := float64(0)
	for i, _ := range c.NetworkMetric{
		for _, delay := range c.NetworkMetric[i]{
			if delay == -1{
				failCount++
			}else{
				connectionCount++
				totalDelay += float64(delay)/1000000.0
			}
		}
	}
	fmt.Printf("Out of %d peers, %d replied, with %d failed measures, the average delay is %f\n",
		peerCount,
		reportCount,
		failCount,
		totalDelay/float64(connectionCount))
}

func (c *ControllerState) report() string {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic Catched in report %s\n", r)
		}
	}()
	state := make([]ProtocolState, 0)
	for i,_ := range c.PeerList{
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			log.Print("Check State:", err.Error())
			continue
		}
		defer client.Close()
		newState := ProtocolState{}
		err = client.Call("ProtocolState.RetrieveState", 1, &newState)
		if err != nil{
			log.Print("Check State RPC:", err.Error())
			continue
		}
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

	// setup watchdog
	go func(){
		for{
			time.Sleep(30 * time.Second)
			c.checkConnection()
		}
	}()
	for {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := scanner.Text()
			switch text {
			case "batch":
				go c.batchTest()
			case "auto":
				go c.autoTest(10, DefaultSetupParams)
			case "setup":
				c.SetupProtocol(1, nil)
			case "start":
				c.StartProtocol(1, nil)
			case "state":
				// pick a random node to retrieve state
				nodeAddr := c.PeerList[rand.Int()%len(c.PeerList)].Address
				print(c.checkState(nodeAddr))
			case "measure":
				c.measure()
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
		c.spawnEvenly(size - len(c.PeerList))
		time.Sleep(10 * time.Second)
	}

	c.SetupParams = params
	c.SetupProtocol(1, nil)
	c.StartProtocol(1, nil)

	print("Auto-test starting!\n")
	var report string
	startTime := time.Now()
	for{
		time.Sleep(10 * time.Second)
		report = c.report()
		if report[0] == 't' || time.Now().Sub(startTime) > 3600 * time.Second{
			// finished
			break
		}
	}
	print("Auto-test completed!\n")
	fmt.Printf("%d, %d, %s", size, len(c.ServerList), report)
}

func (c *ControllerState) batchTest(){
	sizeList := []int {160, 320}
	durationList := []int32 {600}
	deltaList := []float64{0.01,0.005, 0.001}
	fList := []float64{0.01,0.02, 0.03}
	gList := []float64{0.005, 0.01}

	for _, size := range sizeList{
		for _, dur := range durationList{
			for _, delta := range deltaList{
				for _, f := range fList{
					for _, g := range gList{
						for i := 0; i < 1 ; i++{
							c.SetupParams.RoundDuration = time.Duration(dur) * time.Millisecond
							c.SetupParams.Delta = delta
							c.SetupParams.F = f
							c.SetupParams.G = g
							c.autoTest(size, c.SetupParams)
						}
					}
				}
			}
		}
	}
}



func StartServer(exitSignal chan bool){
	c := ControllerState{exitSignal, DefaultSetupParams, make([]message.Identity, 0), "", make([]string, 0)}
	go c.StartListen()
}
