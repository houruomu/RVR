package algorithm

import (
	"RVR/message"
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"
)

// this packet is to monitor and coordinate the nodes

var DefaultSetupParams = ProtocolRPCSetupParams{
	400 * time.Millisecond,
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
	lockHolder    string // completely for debugging purpose
}

func (c *ControllerState) checkConnection() {
	localLock := sync.Mutex{}
	connectedPeers := make([]message.Identity, 0)
	for _, peer := range c.PeerList {
		go func (peer message.Identity){
			err := RpcCall(peer.Address, "ProtocolState.BlackHole", make([]byte, 0), nil, time.Second)
			if err != nil {
				fmt.Printf("Peer %s disconnected.\n", peer.Address)
				return
			}
			localLock.Lock()
			defer localLock.Unlock()
			connectedPeers = append(connectedPeers, peer)
		}(peer)
	}
	time.Sleep(time.Second)

	c.lock.Lock()
	c.lockHolder = "checkConnection"
	c.PeerList = connectedPeers
	c.lockHolder = ""
	c.lock.Unlock()


	connectedServers := make([]string, 0)

	for _, server := range c.ServerList {
		go func (server string){
			err := RpcCall(server, "SpawnerState.BlackHole", make([]byte, 0), nil, time.Second)
			if err != nil {
				fmt.Printf("Server %s disconnected.\n", server)
				return
			}
			localLock.Lock()
			defer localLock.Unlock()
			connectedServers = append(connectedServers, server)
		}(server)
	}

	time.Sleep(time.Second)

	c.lock.Lock()
	c.lockHolder = "checkConnection"
	c.ServerList = connectedServers
	c.lockHolder = ""
	c.lock.Unlock()
}

func (c *ControllerState) spawnEvenly(count int) {
	c.checkConnection()
	c.lock.RLock()
	c.lockHolder = "spawnEvenly"
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
	c.lockHolder = "spawnEvenly"
	newServerList := make([]string, len(c.ServerList))
	perm := rand.Perm(len(c.ServerList))
	for i, v := range perm {
		newServerList[v] = c.ServerList[i]
	}
	c.ServerList = newServerList
	c.lockHolder = ""
	c.lock.Unlock()
}

func (c *ControllerState) Spawn(addr string, count int) {
	RpcCall(addr, "SpawnerState.Spawn", count, nil, time.Second)
}

func (c *ControllerState) RegisterServer(addr string, rtv *int) error {
	c.lock.Lock()
	c.lockHolder = "RegisterServer"
	c.ServerList = append(c.ServerList, addr)
	c.lockHolder = ""
	c.lock.Unlock()
	fmt.Printf("New Server registered at controller: %s\n", addr)
	return nil
}

func (c *ControllerState) Register(id message.Identity, rtv *int) error {
	c.lock.Lock()
	c.lockHolder = "Register"
	c.PeerList = append(c.PeerList, id)
	c.lockHolder = ""
	c.lock.Unlock()
	fmt.Printf("New Peer registered at controller: %s\n", id.Address)
	return nil
}

func (c *ControllerState) setupRandomizedView() error {
	c.lock.RLock()
	c.lockHolder = "setupRandomizedView"
	for i, _ := range c.PeerList {
		view := make([]uint64, 0)
		for j, _ := range c.PeerList {
			if (rand.Float32() < 0.4) {
				// 0.4 is the sample probability for leader's proposal
				view = append(view, c.PeerList[j].GetUUID())
			}
		}
		go RpcCall(c.PeerList[i].Address, "ProtocolState.SetView", view, nil, time.Second)
	}
	c.lock.RUnlock()
	return nil
}

func (c *ControllerState) SetupProtocol(ph1 int, ph2 *int) error {
	c.SetupParams.InitView = c.PeerList
	nEstimate := float64(len(c.PeerList))
	c.SetupParams.X = int(math.Ceil(math.Log(nEstimate)/math.Log(math.Log(nEstimate))+4.0))*c.SetupParams.L + c.SetupParams.Offset

	connectedPeers := make([]message.Identity, 0)
	for _, peer := range c.PeerList {
		err := RpcCall(peer.Address, "ProtocolState.Setup", c.SetupParams, nil, time.Second)
		if err != nil {
			RpcCall(peer.Address, "ProtocolState.Exit", c.SetupParams, nil, time.Second)
		} else {
			connectedPeers = append(connectedPeers, peer)
		}
	}
	c.lock.Lock()
	c.lockHolder = "SetupProtocol"
	c.PeerList = connectedPeers
	c.lock.Unlock()
	c.setupRandomizedView()
	fmt.Printf(c.SetupParams.String())
	return nil
}

func (c *ControllerState) killNode(addr string) {
	RpcCall(addr, "ProtocolState.Exit", 1, nil, time.Second)
}

func (c *ControllerState) KillNodes(ph1 int, ph2 *int) error {
	c.lock.RLock()
	c.lockHolder = "KillNodes"
	defer c.lock.RUnlock()
	for i, _ := range c.PeerList {
		c.killNode(c.PeerList[i].Address)
	}
	return nil
}

func (c *ControllerState) KillServers(ph1 int, ph2 *int) error {
	for i, _ := range c.ServerList {
		RpcCall(c.ServerList[i], "SpawnerState.Exit", 1, nil, time.Second)
	}
	return nil
}

func (c *ControllerState) StartProtocol(ph1 int, ph2 *int) error {
	c.lock.RLock()
	c.lockHolder = "StartProtocol"
	for i, _ := range c.PeerList {
		go RpcCall(c.PeerList[i].Address, "ProtocolState.Start", 1, nil, c.SetupParams.RoundDuration)
	}
	c.lock.RUnlock()

	time.Sleep(time.Duration(c.SetupParams.Offset) * c.SetupParams.RoundDuration)
	startedPeers := make([]message.Identity, 0)
	for _, peer := range c.PeerList {
		state := ProtocolState{}
		err := RpcCall(peer.Address, "ProtocolState.RetrieveState", 1, &state, c.SetupParams.RoundDuration)
		if err == nil && state.Round > 1 {
			startedPeers = append(startedPeers, peer)
		} else {
			c.killNode(peer.Address)
		}
	}
	c.lock.Lock()
	c.lockHolder = "StartProtocol"
	c.PeerList = startedPeers
	c.lock.Unlock()
	return nil
}

func (c *ControllerState) checkState(address string) string {
	state := ProtocolState{}
	RpcCall(address, "ProtocolState.RetrieveState", 1, &state, c.SetupParams.RoundDuration)
	return state.String()
}

func (c *ControllerState) AcceptReport(report PingValueReport, ph2 *int) error {
	c.lock.Lock()
	c.lockHolder = "AcceptReport"
	c.NetworkMetric = append(c.NetworkMetric, report)
	c.lock.Unlock()
	return nil
}

func (c *ControllerState) measure() {
	c.NetworkMetric = make([]PingValueReport, 0)
	c.lock.RLock()
	c.lockHolder = "measure"
	for _, peer := range c.PeerList {
		go RpcCall(peer.Address, "ProtocolState.PingReport", 2000, nil, c.SetupParams.RoundDuration)
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

func (c *ControllerState) report() (report string, fin bool, cons bool, round int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic Catched in report %s\n", r)
		}
	}()
	statelen := 0
	stateChan := make(chan *ProtocolState)
	for _, peer := range c.PeerList {
		statelen++
		go func (addr string){
			newState := ProtocolState{}
			err := RpcCall(addr, "ProtocolState.RetrieveState", 1, &newState, c.SetupParams.RoundDuration)
			if err != nil {
				fmt.Printf("report state error: %s \n", err.Error())
				stateChan <- nil
			}else{
				stateChan <- &newState
			}
		}(peer.Address)
	}
	state := make([]ProtocolState, 0)
	for i := 0; i < statelen; i++{
		s := <-stateChan
		if s != nil{
			state = append(state, *s)
		}
	}
	if len(state) != len(c.PeerList){
		return "f", false, false, -1
	}

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
				go c.autoTest(10, DefaultSetupParams, false)
			case "setup":
				go c.SetupProtocol(1, nil)
			case "start":
				go c.StartProtocol(1, nil)
			case "state":
				go func() {
					// pick a random node to retrieve state
					nodeAddr := c.PeerList[rand.Int()%len(c.PeerList)].Address
					fmt.Printf(c.checkState(nodeAddr))
				}()
			case "lock":
				fmt.Printf("Lock holder: %s\n", c.lockHolder)
			case "measure":
				go c.measure()
			case "report":
				// pick a random node to retrieve state
				go func() {
					report, _, _, _ := c.report()
					fmt.Printf("%s\n", report)
				}()
			case "reset":
				go func() {
					c.KillNodes(1, nil)
					c.PeerList = make([]message.Identity, 0)
				}()
			case "load":
				go func(){
					c.lock.Lock(); c.ServerList = SPAWNER_LIST; c.lock.Unlock(); fmt.Printf("default spawners loaded.\n")
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

func (c *ControllerState) autoTest(size int, params ProtocolRPCSetupParams, stopOnceConsensus bool) (consensus bool) {
	c.KillNodes(1, nil)
	c.PeerList = make([]message.Identity, 0)
	for len(c.PeerList) < size {
		c.spawnEvenly(size - len(c.PeerList))
		time.Sleep(10 * time.Second)
	}

	c.SetupParams = params
	fmt.Printf("Auto test: setting up protocol\n", )
	c.SetupProtocol(1, nil)

	time.Sleep(3 * time.Second)

	fmt.Printf("Auto test: starting protocol\n", )
	c.StartProtocol(1, nil)

	fmt.Printf("Auto-test started!\n")
	var report string
	consensusReached := false
	consensusTime := 3600 * time.Second
	consensusRound := 0
	startTime := time.Now()
	for !(stopOnceConsensus && consensusReached) {
		time.Sleep(10 * time.Second)
		_report, fin, cons, round := c.report()
		report = _report
		if !consensusReached && cons {
			consensusReached = true
			consensusTime = time.Now().Sub(startTime)
			consensusRound = round
		}
		if fin || time.Now().Sub(startTime) > 2000*time.Second {
			// finished
			break
		}
	}
	fmt.Printf("Auto-test completed!\n")
	fmt.Printf("%d, %d, %s, %d, %d\n", len(c.PeerList), len(c.ServerList), report, consensusTime, consensusRound)
	return consensusReached
}

func (c *ControllerState) batchTest() {
	sizeList := []int{160, 80}
	durationList := []int32{100, 200, 400}
	deltaList := []float64{0.005, 0.01, 0.001}
	fList := []float64{0.03, 0.01, 0.02}
	gList := []float64{0.005, 0.01, 0.0025}
	offsetList := []int{2, 4, 6, 8}

	//for _, size := range sizeList {
	//	for _, dur := range durationList {
	//		for _, delta := range deltaList {
	//			for _, f := range fList {
	//				for _, g := range gList {
	//					for i := 0; i < 1; i++ {
	//						c.SetupParams.RoundDuration = time.Duration(dur) * time.Millisecond
	//						c.SetupParams.Delta = delta
	//						c.SetupParams.F = f
	//						c.SetupParams.G = g
	//						c.autoTest(size, c.SetupParams)
	//					}
	//				}
	//			}
	//		}
	//	}
	//}

	for i := 0; i < 5; i++ {
		c.SetupParams = DefaultSetupParams
		for _, size := range sizeList {
			c.autoTest(size, c.SetupParams, true)
		}
		c.SetupParams = DefaultSetupParams
		for _, delta := range deltaList {
			c.SetupParams.Delta = delta
			c.autoTest(160, c.SetupParams, false)
		}
		c.SetupParams = DefaultSetupParams
		for _, dur := range durationList {
			for _, offset := range offsetList {
				c.SetupParams.Offset = offset
				c.SetupParams.RoundDuration = time.Duration(dur) * time.Millisecond
				if c.autoTest(160, c.SetupParams, true) {
					break
				}
			}
		}
		c.SetupParams = DefaultSetupParams
		for _, f := range fList {
			c.SetupParams.F = f
			c.autoTest(160, c.SetupParams, false)
		}
		c.SetupParams = DefaultSetupParams
		for _, g := range gList {
			c.SetupParams.G = g
			c.autoTest(160, c.SetupParams, false)
		}
	}
}

func StartServer(exitSignal chan bool) {
	c := ControllerState{}
	c.ExitSignal = exitSignal
	c.SetupParams = DefaultSetupParams
	go c.StartListen()
}
