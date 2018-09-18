package algorithm

import (
	"RVR/message"
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"net/rpc"
	"os"
	"strings"
	"sync"
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
	ExitSignal    chan bool
	SetupParams   ProtocolRPCSetupParams
	PeerList      []message.Identity
	Address       string
	ServerList    []string
	NetworkMetric []PingValueReport
	lock          sync.RWMutex
}

func (c *ControllerState) checkConnection() {
	connectedPeers := make([]message.Identity, 0)
	c.lock.RLock()
	for i, _ := range c.PeerList {
		err := RpcCall(c.PeerList[i].Address, "ProtocolState.BlackHole", make([]byte, 0), nil)
		if err != nil {
			fmt.Printf("Peer %s disconnected.\n", c.PeerList[i].Address)
			continue
		}
		connectedPeers = append(connectedPeers, c.PeerList[i])
	}
	c.lock.RUnlock()
	c.lock.Lock()
	c.PeerList = connectedPeers
	c.lock.Unlock()
	connectedServers := make([]string, 0)
	c.lock.RLock()
	for i, _ := range c.ServerList {
		err := RpcCall(c.ServerList[i], "SpawnerState.BlackHole", make([]byte, 0), nil)
		if err != nil {
			fmt.Printf("Server %s disconnected.\n", c.ServerList[i])
			continue
		}
		connectedServers = append(connectedServers, c.ServerList[i])
	}
	c.lock.RUnlock()
	c.lock.Lock()
	c.ServerList = connectedServers
	c.lock.Unlock()
}

func (c *ControllerState) spawnEvenly(count int) {
	c.checkConnection()
	c.lock.RLock()
	for i, _ := range c.ServerList {
		numInstance := count / len(c.ServerList)
		if i < count%len(c.ServerList) {
			numInstance++
		}
		c.Spawn(c.ServerList[i], numInstance)
	}
	c.lock.RUnlock()
	// shuffle the list //https://stackoverflow.com/questions/12264789/shuffle-array-in-go
	c.lock.Lock()
	newServerList := make([]string, len(c.ServerList))
	perm := rand.Perm(len(c.ServerList))
	for i, v := range perm {
		newServerList[v] = c.ServerList[i]
	}
	c.ServerList = newServerList
	c.lock.Unlock()
}

func (c *ControllerState) Spawn(addr string, count int) {
	RpcCall(addr, "SpawnerState.Spawn", count, nil)
}

func (c *ControllerState) RegisterServer(addr string, rtv *int) error {
	c.lock.Lock()
	c.ServerList = append(c.ServerList, addr)
	c.lock.Unlock()
	fmt.Printf("New Server registered at controller: %s\n", addr)
	return nil
}

func (c *ControllerState) Register(id message.Identity, rtv *int) error {
	c.lock.Lock()
	c.PeerList = append(c.PeerList, id)
	c.lock.Unlock()
	fmt.Printf("New Peer registered at controller: %s\n", id.Address)
	return nil
}

func (c *ControllerState) setupRandomizedView() error {
	c.lock.RLock()
	for i, _ := range c.PeerList {
		view := make([]uint64, 0)
		for j, _ := range c.PeerList {
			if (rand.Float32() < 0.4) {
				// 0.4 is the sample probability for leader's proposal
				view = append(view, c.PeerList[j].GetUUID())
			}
		}
		go RpcCall(c.PeerList[i].Address, "ProtocolState.SetView", view, nil)
	}
	c.lock.RUnlock()
	return nil
}

func (c *ControllerState) SetupProtocol(ph1 int, ph2 *int) error {
	c.SetupParams.InitView = c.PeerList
	nEstimate := float64(len(c.PeerList))
	c.SetupParams.X = int(math.Ceil(math.Log(nEstimate)/math.Log(math.Log(nEstimate))+4.0))*c.SetupParams.L + c.SetupParams.Offset

	connectedPeers := make([]message.Identity, 0)
	c.lock.RLock()
	for i, _ := range c.PeerList {
		err := RpcCall(c.PeerList[i].Address, "ProtocolState.Setup", c.SetupParams, nil)
		if err != nil {
			RpcCall(c.PeerList[i].Address, "ProtocolState.Exit", c.SetupParams, nil)
		} else {
			connectedPeers = append(connectedPeers, c.PeerList[i])
		}
	}
	c.lock.RUnlock()
	c.lock.Lock()
	c.PeerList = connectedPeers
	c.lock.Unlock()
	c.setupRandomizedView()
	print(c.SetupParams.String())
	return nil
}

func (c *ControllerState) killNode(addr string) {
	RpcCall(addr, "ProtocolState.Exit", 1, nil)
}

func (c *ControllerState) KillNodes(ph1 int, ph2 *int) error {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for i, _ := range c.PeerList {
		c.killNode(c.PeerList[i].Address)
	}
	return nil
}

func (c *ControllerState) KillServers(ph1 int, ph2 *int) error {
	for i, _ := range c.ServerList {
		RpcCall(c.ServerList[i], "SpawnerState.Exit", 1, nil)
	}
	return nil
}

func (c *ControllerState) StartProtocol(ph1 int, ph2 *int) error {
	callList := make([]*rpc.Call, len(c.PeerList))
	c.lock.RLock()
	for i, _ := range c.PeerList {
		client, err := rpc.Dial("tcp", c.PeerList[i].Address)
		if err != nil {
			fmt.Printf("Starting Protocol:", err.Error())
			return nil
		}
		defer client.Close()
		callList[i] = client.Go("ProtocolState.Start", 1, nil, nil)
	}
	c.lock.RUnlock()

	time.Sleep(time.Duration(c.SetupParams.Offset) * c.SetupParams.RoundDuration)
	startedPeers := make([]message.Identity, 0)
	for i, call := range callList {
		select {
		case <-call.Done:
			startedPeers = append(startedPeers, c.PeerList[i])
		default:
			c.killNode(c.PeerList[i].Address)
		}
	}
	c.lock.Lock()
	c.PeerList = startedPeers
	c.lock.Unlock()
	return nil
}

func (c *ControllerState) checkState(address string) string {
	state := ProtocolState{}
	RpcCall(address, "ProtocolState.RetrieveState", 1, &state)
	return state.String()
}

func (c *ControllerState) AcceptReport(report PingValueReport, ph2 *int) error {
	c.lock.Lock()
	c.NetworkMetric = append(c.NetworkMetric, report)
	c.lock.Unlock()
	return nil
}

func (c *ControllerState) measure() {
	c.NetworkMetric = make([]PingValueReport, 0)
	c.lock.RLock()
	for i, _ := range c.PeerList {
		go RpcCall(c.PeerList[i].Address, "ProtocolState.PingReport", 2000, nil)
	}
	c.lock.RUnlock()

	time.Sleep(6 * c.SetupParams.RoundDuration)

	// process
	peerCount := len(c.PeerList)
	reportCount := len(c.NetworkMetric)
	connectionCount := 0
	failCount := 0
	totalDelay := float64(0)
	for i, _ := range c.NetworkMetric {
		for _, delay := range c.NetworkMetric[i] {
			if delay <= 0 {
				failCount++
			} else {
				connectionCount++
				totalDelay += float64(delay) / 1000000.0
			}
		}
	}
	var avgDelay float64
	if connectionCount == 0 {
		avgDelay = -1
	} else {
		avgDelay = totalDelay / float64(connectionCount)
	}
	fmt.Printf("Out of %d peers, %d replied, with %d failed measures, the average delay is %f\n",
		peerCount,
		reportCount,
		failCount,
		avgDelay)
}

func (c *ControllerState) report() string {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic Catched in report %s\n", r)
		}
	}()
	state := make([]ProtocolState, 0)
	c.lock.RLock()
	for i, _ := range c.PeerList {
		newState := ProtocolState{}
		err := RpcCall(c.PeerList[i].Address, "ProtocolState.RetrieveState", 1, &newState)
		if err != nil {
			fmt.Printf("Check State error RPC:", err.Error())
			continue
		}
		state = append(state, newState)
	}
	c.lock.RUnlock()
	analysis := Data{state, c.SetupParams}
	return analysis.Report()
}

func (c *ControllerState) StartListen() {
	c.PeerList = make([]message.Identity, 0)

	portString := ListenRPC(":9696", c, c.ExitSignal)
	myPort := portString[strings.LastIndex(portString, ":"):]
	myIP := GetOutboundAddr()
	c.Address = myIP + myPort
	fmt.Printf("Controller started at Address: %s\n", c.Address)

	// setup watchdog
	go func() {
		for {
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
				go func(){
					// pick a random node to retrieve state
					nodeAddr := c.PeerList[rand.Int()%len(c.PeerList)].Address
					print(c.checkState(nodeAddr))
				}()
			case "measure":
				go c.measure()
			case "report":
				// pick a random node to retrieve state
				go func(){print(c.report())}()
			case "reset":
				go func(){
					c.KillNodes(1, nil)
					c.PeerList = make([]message.Identity, 0)
				}()
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

func (c *ControllerState) autoTest(size int, params ProtocolRPCSetupParams) {
	c.KillNodes(1, nil)
	c.PeerList = make([]message.Identity, 0)
	for len(c.PeerList) < size {
		c.spawnEvenly(size - len(c.PeerList))
		time.Sleep(10 * time.Second)
	}

	c.SetupParams = params
	c.SetupProtocol(1, nil)
	c.StartProtocol(1, nil)

	print("Auto-test starting!\n")
	var report string
	startTime := time.Now()
	for {
		time.Sleep(10 * time.Second)
		report = c.report()
		if report == ""{
			continue
		}
		if report[0] == 't' || time.Now().Sub(startTime) > 2000*time.Second {
			// finished
			break
		}
	}
	print("Auto-test completed!\n")
	fmt.Printf("%d, %d, %s", len(c.PeerList), len(c.ServerList), report)
}

func (c *ControllerState) batchTest() {
	sizeList := []int{160, 320}
	durationList := []int32{600}
	deltaList := []float64{0.01, 0.005, 0.001}
	fList := []float64{0.01, 0.02, 0.03}
	gList := []float64{0.005, 0.01}

	for _, size := range sizeList {
		for _, dur := range durationList {
			for _, delta := range deltaList {
				for _, f := range fList {
					for _, g := range gList {
						for i := 0; i < 1; i++ {
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

func StartServer(exitSignal chan bool) {
	c := ControllerState{}
	c.ExitSignal = exitSignal
	c.SetupParams = DefaultSetupParams
	go c.StartListen()
}
