package algorithm

import (
	"RVR/message"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"math"
	rand2 "math/rand"
	"net"
	"net/rpc"
	"strings"
	"sync"
	"time"
)

// TODO: For each RPC connection, use persistent connection, if there are too many of them, use a pool
type ProtocolState struct {
	// protocol parameters
	privateKey     *rsa.PrivateKey
	roundDuration  time.Duration
	offset         int
	f              float64
	g              float64
	l              int
	x              int // The number of rounds for Gossip to run
	delta          float64
	initView       []message.Identity
	ControlAddress string

	// protocol state
	Round        int
	inQueue      []message.Message
	View         []uint64
	lock         sync.RWMutex
	ticker       <-chan time.Time
	idToAddrMap  map[uint64]string // use to check whether in initview
	MyId         message.Identity
	Finished     bool
	ExitSignal   chan bool
	StartTime    time.Time
	FinishTime   time.Time
	CurrentProto string

	// protocol measurement data
	Malicious      bool
	MsgCount       int
	ByteCount      int
	LargestMsgSize int
	MsgReceived    int
	PingEstimate   float64 // use filter to estimate ping
	FailToSend     int
	ExpiredMsg     int
}

type ProtocolRPCSetupParams struct {
	RoundDuration time.Duration
	Offset        int
	F             float64
	G             float64
	L             int
	X             int // The number of rounds for Gossip to run
	Delta         float64
	Id            message.Identity
	InitView      []message.Identity
}

func (p *ProtocolRPCSetupParams) String() string {
	// print the current state summary
	return fmt.Sprintf("-----------------------\n"+
		"Using parameter:\n"+
		"RoundDuration: %dms\n"+
		"F: %f\n"+
		"G: %f\n"+
		"L: %d\n"+
		"X: %d\n"+
		"Delta: %f\n"+
		"-----------------------\n",
		p.RoundDuration/time.Millisecond, p.F, p.G, p.L, p.X, p.Delta)
}

func (p *ProtocolState) String() string {
	// print the current state summary
	return fmt.Sprintf("-----------------------\n"+
		"State of node %X @ %s:\n"+
		"Round: %d\n"+
		"Finished: %t\n"+
		"message sent: %d\n"+
		"bytes sent: %d\n"+
		"largest message size: %d\n"+
		"message received: %d\n"+
		"view size: %d\n"+
		"CurrentProto: %s\n"+
		"-----------------------\n",
		p.MyId.GetUUID(), p.MyId.Address, p.Round, p.Finished, p.MsgCount, p.ByteCount, p.LargestMsgSize, p.MsgReceived, len(p.View), p.CurrentProto)
}

func GetOutboundAddr() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		fmt.Printf("Unable to get outbound address.\n", )
		log.Fatal(err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func (p *ProtocolState) testPing(size int, addr string, timeout time.Duration, rtv chan int) int {
	data := make([]byte, size)
	rand.Read(data)
	startTime := time.Now()
	err := RpcCall(addr, "ProtocolState.BlackHole", data, nil, timeout)
	if err != nil {
		if rtv != nil {
			rtv <- -1
		}
		return -1
	} else {
		finTime := int(time.Now().Sub(startTime))
		if rtv != nil {
			rtv <- finTime
		}
		return finTime
	}
}

func (p *ProtocolState) pingReport(size int) PingValueReport {
	// this function returns an array of ping values to every peer
	reportLen := 0
	reportChan := make(chan int, 0)
	for _, peer := range p.initView {
		reportLen++
		go func(addr string) {
			p.testPing(size, addr, p.roundDuration, reportChan)
		}(peer.Address)
	}
	report := make(PingValueReport, reportLen)
	for i := 0; i < reportLen; i++ {
		report[i] = <-reportChan
	}
	return report
}

func (p *ProtocolState) PingReport(size int, rtv *int) error {
	go func() {
		report := p.pingReport(size)
		RpcCall(p.ControlAddress, "ControllerState.AcceptReport", report, nil, p.roundDuration)
	}()
	return nil
}

func (p *ProtocolState) localMonitor(recur int) bool {
	if recur == 0 {
		p.Malicious = true
		return false
	}
	report := p.pingReport(2000)
	failure := 0
	for i, _ := range report {
		if report[i] <= 0 || report[i] >= int(p.roundDuration) {
			failure++
		}
	}
	if float64(failure) > (float64(len(p.initView))*p.g + 5) {
		fmt.Printf("%s: bad round, recur = %d, failure count: %d\n", p.MyId.Address, recur, failure)
		time.Sleep(p.roundDuration)
		return p.localMonitor(recur - 1)
	} else {
		return true
	}
}

//func (p *ProtocolState) peerMonitor() {
//	newPeerList := make([]message.Identity, 0)
//	peerChan := make(chan *message.Identity)
//	listLen := 0
//	// no need lock since we are dealing with values only
//	for _, peer := range p.initView {
//		listLen++
//		go func(peer message.Identity) {
//			tic := time.Tick(p.roundDuration)
//			for i := 0; i < p.l; i++ {
//				<-tic
//				err := RpcCall(peer.Address, "ProtocolState.BlackHole", make([]byte, 1), nil, p.roundDuration)
//				if err == nil {
//					peerChan <- &peer
//					break
//				}
//			}
//			fmt.Printf("dropping peer %s\n", peer.Address)
//			delete(p.idToAddrMap, peer.GetUUID())
//			peerChan <- nil
//		}(peer)
//	}
//	for i := 0; i < listLen; i++ {
//		p := <-peerChan
//		if p != nil {
//			newPeerList = append(newPeerList, *p)
//		}
//	}
//	p.lock.Lock()
//	p.initView = newPeerList
//	p.lock.Unlock()
//}

func (p *ProtocolState) BlackHole(msg []byte, rtv *int) error {
	// this is a blackhole function for measuring ping value
	for i, _ := range msg {
		if msg[i] == 0 {
			return nil
		}
	}
	return nil
}

func (p *ProtocolState) SendInMsg(msg message.Message, rtv *int) error {
	p.lock.RLock()
	if (msg.Round < p.Round-p.offset) {
		p.lock.RUnlock()
		p.lock.Lock()
		p.ExpiredMsg++
		p.lock.Unlock()
		return errors.New("Trying to enroll an expired msg")
	}
	p.lock.RUnlock()
	err := msg.Verify()
	if err != nil {
		return err
	}
	if _, ok := p.idToAddrMap[msg.Sender.GetUUID()]; !ok {
		return fmt.Errorf("Not in initview.\n")
	}
	p.lock.Lock()
	defer p.lock.Unlock()
	if msg.Round > p.Round+p.offset && p.CurrentProto != "Gossip" {
		fmt.Printf("%s: unsynchronized, Malicious, at round %d, received msg at round %d from %s\n",
			p.MyId.Address, p.Round, msg.Round, msg.Sender.Address)
		p.Malicious = true
	} else {
		p.inQueue = append(p.inQueue, msg)
		p.MsgReceived++
	}
	return nil
}

func (p *ProtocolState) init() {
	// init parameters
	var err error
	p.privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	p.roundDuration = 100 * time.Millisecond
	p.offset = 3
	p.f = 0.01  // < 0.1
	p.g = 0.005 // <0.01
	p.l = 2
	p.delta = 0.01
	// p.x is only updated when the initview is updated

	// init states
	p.Round = 1
	p.inQueue = make([]message.Message, 0)
	p.initView = make([]message.Identity, 0)
	p.View = make([]uint64, 0)
	p.idToAddrMap = make(map[uint64]string)
	p.ticker = time.Tick(p.roundDuration) // TODO: use a separate function to start ticker

	// init the rpc server
	handler := rpc.NewServer() // allows multiple rpc at a time
	handler.Register(p)
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
	p.MyId = message.Identity{myIP + myPort, x509.MarshalPKCS1PublicKey(&p.privateKey.PublicKey)}
	p.initView = append(p.initView, p.MyId)
	p.idToAddrMap[p.MyId.GetUUID()] = p.MyId.Address
	for _, id := range p.initView {
		p.idToAddrMap[id.GetUUID()] = id.Address
	}

	log.Printf("RPC Server started, Listening on %s", p.MyId.Address)
} // this function starts the protocol at once

func (p *ProtocolState) GetReady() {
	// this function setup the server in a waiting-for-instruct phase
	// setup private keys
	var err error
	p.privateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	// setup RPC server
	myIP := GetOutboundAddr()
	portString := ListenRPC(":0", p, p.ExitSignal)
	myPort := portString[strings.LastIndex(portString, ":"):]

	p.MyId = message.Identity{myIP + myPort, x509.MarshalPKCS1PublicKey(&p.privateKey.PublicKey)}

	// report to controller
	RpcCall(p.ControlAddress, "ControllerState.Register", p.MyId, nil, p.roundDuration*time.Duration(p.l))
	fmt.Printf("Node ready to receive instructions, Address: %s\n", p.MyId.Address)

}

func (p *ProtocolState) Setup(state ProtocolRPCSetupParams, rtv *int) error {
	// this function sets up the server
	// setup parameters based on the incoming instruction
	// copy the state parameters
	p.roundDuration = state.RoundDuration
	p.offset = state.Offset
	p.f = state.F
	p.g = state.G
	p.l = state.L
	p.x = state.X
	p.delta = state.Delta
	p.initView = state.InitView
	// initialize the state parameters
	p.Round = 0
	p.inQueue = make([]message.Message, 0)
	p.View = make([]uint64, 0)
	p.idToAddrMap = make(map[uint64]string)

	for i, _ := range p.initView {
		p.idToAddrMap[p.initView[i].GetUUID()] = p.initView[i].Address
	}

	fmt.Printf("Node setup done, initview length: %d\n", len(p.initView))

	return nil
}

func (p *ProtocolState) SetView(view []uint64, rtv *int) error {
	p.View = view
	return nil
}

func (p *ProtocolState) Start(command int, rtv *int) error {
	// this function starts the algorithm
	// start the ticker
	p.ticker = time.Tick(p.roundDuration)
	// invoke View Reconciliation (asynchrously)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// fail gracefully
				p.Finished = true
				p.FinishTime = time.Now()
				p.Exit(1, nil)
			}
		}()
		p.viewReconciliation()
	}()
	p.StartTime = time.Now()
	go func() {
		for !p.Finished && !p.Malicious {
			time.Sleep(10 * time.Second)
			select {
			case <-p.ExitSignal:
				p.ExitSignal <- true
				return
			default:
			}
			//p.peerMonitor()
			p.localMonitor(p.l)
		}
		if p.Malicious{
			fmt.Printf("%s is marked as Malicious. \n", p.MyId.Address)
		}else{
			fmt.Printf("%s finished. \n", p.MyId.Address)
		}
	}()
	return nil
}

func (p *ProtocolState) Exit(command int, rtv *int) error {
	p.ExitSignal <- true
	return nil
}

func (p *ProtocolState) RetrieveState(ph int, state *ProtocolState) error {
	// returns the current state of the node to the controller
	*state = *p
	return nil
}

func (p *ProtocolState) addToInitView(id message.Identity) {
	if _, ok := p.idToAddrMap[id.GetUUID()]; !ok {
		p.initView = append(p.initView, id)
		p.idToAddrMap[id.GetUUID()] = id.Address
		nEstimate := float64(len(p.initView))
		p.x = int(math.Ceil(math.Log(nEstimate)/math.Log(math.Log(nEstimate))+4.0))*p.l + p.offset
	}
}

func (p *ProtocolState) removeFromInitView(addr string) {
	var pos = 0
	found := false
	for pos, _ = range p.initView {
		if p.initView[pos].Address == addr {
			found = true
			break
		}
	}
	if found {
		delete(p.idToAddrMap, p.initView[pos].GetUUID())
		p.initView = append(p.initView[:pos], p.initView[pos+1:]...)
	}

}
func (p *ProtocolState) sendMsgToPeerAsync(m message.Message, addr string) {
	go func() {
		err := RpcCall(addr, "ProtocolState.SendInMsg", m, nil, p.roundDuration)
		// measurement
		if err != nil {
			p.lock.Lock()
			p.FailToSend++
			if err.Error() == "Not in initview.\n" {
				p.removeFromInitView(addr)
			}
			p.lock.Unlock()
		} else {
			p.lock.Lock()
			p.MsgCount++
			size := int(m.Size())
			p.ByteCount += size
			if p.LargestMsgSize < size {
				p.LargestMsgSize = size
			}
			p.lock.Unlock()
		}
	}()
}

func (p *ProtocolState) sendMsgToPeerWithTrial(m message.Message, addr string, trial int) error {
	if trial <= 0 {
		return fmt.Errorf("Fail to send to %s.\n", addr)
	}
	err := RpcCall(addr, "ProtocolState.SendInMsg", m, nil, p.roundDuration)
	// measurement
	if err != nil {
		p.lock.Lock()
		p.FailToSend++
		p.lock.Unlock()
		return p.sendMsgToPeerWithTrial(m, addr, trial-1)
	} else {
		p.lock.Lock()
		p.MsgCount++
		size := int(m.Size())
		p.ByteCount += size
		if p.LargestMsgSize < size {
			p.LargestMsgSize = size
		}
		p.lock.Unlock()
		return nil
	}
}

func (p *ProtocolState) updateWithPeers(peers []string, maxRound int) {
	// every Round advertise one of my peer to all my peers
	// succeed if heard from every one
	for totalRound := maxRound; totalRound > 0; totalRound-- {
		<-p.ticker // Round counter
		p.lock.Lock()
		p.Round++
		// process all messages
		for _, m := range p.inQueue {
			if addr, ok := p.idToAddrMap[m.Sender.GetUUID()]; !ok || addr != m.Sender.Address {
				p.idToAddrMap[m.Sender.GetUUID()] = m.Sender.Address
				if !ok {
					p.initView = append(p.initView, m.Sender)
				}
			}
		}
		p.inQueue = make([]message.Message, 0)
		p.lock.Unlock()

		m := new(message.Message)
		m.Sender = p.MyId
		// since I have changed the way it works, we need to broadcast a random guy to all peers
		m.Sender = p.initView[rand2.Int()%len(p.initView)]

		p.lock.RLock()
		m.Round = p.Round
		p.lock.RUnlock()

		m.Sign(p.privateKey)
		for _, addr := range peers {
			p.sendMsgToPeerAsync(*m, addr)
		}
		for _, addr := range p.idToAddrMap {
			p.sendMsgToPeerAsync(*m, addr)
		}
	}
}

func (p *ProtocolState) sketch() (round int, duration time.Duration){
	// sketch the run and return the computed duration
	repetity := int(6.0*math.Log(2/p.delta) + 1)
	round = repetity * (1 + electionSketch(p, 1) + sampleSketch(p) + gossipSketch(p))
	duration = time.Duration(round) * p.roundDuration
	return
}

func (p *ProtocolState) viewReconciliation() {
	fmt.Printf("%s starting RVR protocol, initial View length: %d\n", p.MyId.Address, len(p.View))
	repetity := int(6.0*math.Log(2/p.delta) + 1)
	// repetity /= 32
	for i := 0; i < repetity; i++ {
		select {
		case <-p.ExitSignal:
			p.ExitSignal <- true
			return
		default:

		}

		p.lock.Lock()
		<-p.ticker
		p.Round++
		p.lock.Unlock()

		p.lock.Lock()
		p.CurrentProto = "Election"
		p.lock.Unlock()
		leader := DoElection(p, 1)
		if leader.Public_key == nil{
			fmt.Printf("Leader Election failed, round %d.\n", p.Round)
		}else{
			fmt.Printf("Leader Election succeeded, round %d.\n", p.Round )
		}

		p.lock.Lock()
		p.CurrentProto = "Sample"
		p.lock.Unlock()
		scores := Sample(p)
		if bytes.Equal(leader.Public_key, p.MyId.Public_key) {
			if scores != nil {
				p.View = make([]uint64, 0)
				for uuid, score := range scores {
					if score > 0.4 {
						p.View = append(p.View, uuid)
					}
				}
			}
		}

		p.lock.Lock()
		p.CurrentProto = "Gossip"
		p.lock.Unlock()
		proposal := Gossip(p, &leader)

		p.lock.Lock()
		p.CurrentProto = "Compute"
		p.lock.Unlock()
		// build a check map
		proposalMap := make(map[uint64]bool)
		for _, uuid := range proposal {
			proposalMap[uuid] = true
		}
		if scores != nil {
			if proposal != nil {
				p.View = make([]uint64, 0)
				for uuid, score := range scores {
					if score > 0.65 || (score >= 0.16 && proposalMap[uuid]) {
						p.View = append(p.View, uuid)
					}
				}
			} else {
				newView := make([]uint64, 0)
				for uuid, score := range scores {
					if score > 0.65 {
						newView = append(newView, uuid)
					}
				}
				for _, uuid := range p.View {
					if scores[uuid] <= 0.65 && scores[uuid] >= 0.16 {
						newView = append(newView, uuid)
					}
				}
				p.View = newView
			}
		}
	}
	fmt.Printf("%s finishing RVR protocol, final View length: %d\n", p.MyId.Address, len(p.View))
	p.Finished = true
	p.FinishTime = time.Now()
	pRound, pTime := p.sketch()
	fmt.Printf("Proposed Rounds: %d, actual rounds: %d; proposed time %s, actual time %s\n", pRound, p.Round, pTime, p.FinishTime.Sub(p.StartTime) )
}

func StartNode(controlAddress string, exitSignal chan bool) *ProtocolState {
	p := ProtocolState{}
	p.ControlAddress = controlAddress
	p.ExitSignal = exitSignal
	go p.GetReady()
	return &p
}
