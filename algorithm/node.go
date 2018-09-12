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
	privateKey    *rsa.PrivateKey
	roundDuration time.Duration
	offset        int
	f             float64
	g             float64
	l             int
	x             int // The number of rounds for Gossip to run
	delta         float64
	initView      []message.Identity

	// protocol state
	Round          int
	inQueue        []message.Message
	View           []uint64
	lock           sync.RWMutex
	ticker         <-chan time.Time
	idToAddrMap    map[uint64]string // use to check whether in initview
	MyId           message.Identity
	Finished       bool
	ExitSignal     chan bool
	ControlAddress string
	StartTime      time.Time
	FinishTime     time.Time

	// protocol measurement data
	MsgCount       int
	ByteCount      int
	LargestMsgSize int
	MsgReceived    int
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

func GetOutboundAddr() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func (p *ProtocolState) SendInMsg(msg message.Message, rtv *int) error {
	p.lock.RLock()
	if (msg.Round < p.Round-p.offset) {
		p.lock.RUnlock()
		return errors.New("Trying to enroll an expired msg")
	}
	p.lock.RUnlock()
	err := msg.Verify()
	if err != nil {
		return err
	}
	p.lock.Lock()
	p.inQueue = append(p.inQueue, msg)
	p.MsgReceived++
	p.lock.Unlock()
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
	// report to controller
	client, err := rpc.Dial("tcp", p.ControlAddress)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer client.Close()
	fmt.Printf("Node ready to receive instructions, Address: %s\n", p.MyId.Address)
	client.Go("ControllerState.Register", p.MyId, nil, nil)
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
	p.Round = 1
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
		defer func(){
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
		"-----------------------\n",
		p.MyId.GetUUID(), p.MyId.Address, p.Round, p.Finished, p.MsgCount, p.ByteCount, p.LargestMsgSize, p.MsgReceived, len(p.View))
}

func (p *ProtocolState) addToInitView(id message.Identity) {
	if _, ok := p.idToAddrMap[id.GetUUID()]; !ok {
		p.initView = append(p.initView, id)
		p.idToAddrMap[id.GetUUID()] = id.Address
		nEstimate := float64(len(p.initView))
		p.x = int(math.Ceil(math.Log(nEstimate)/math.Log(math.Log(nEstimate))+4.0))*p.l + p.offset
	}
}

func (p *ProtocolState) sendMsgToPeerAsync(m message.Message, addr string) {
	go func() {
		client, err := rpc.Dial("tcp", addr)
		if err != nil {
			log.Print("dialing:", err.Error())
			return
		}
		defer client.Close()
		client.Go("ProtocolState.SendInMsg", m, nil, nil)

		// measurement
		p.MsgCount++
		size := int(m.Size())
		p.ByteCount += size
		if p.LargestMsgSize < size {
			p.LargestMsgSize = size
		}
	}()
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

func (p *ProtocolState) viewReconciliation() {
	fmt.Printf("%s starting RVR protocol, initial View length: %d\n", p.MyId.Address, len(p.View))
	repetity := int(6.0*math.Log(2/p.delta) + 1)
	// repetity /= 32
	for i := 0; i < repetity; i++ {
		leader := DoElection(p, 1)
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
		proposal := Gossip(p, &leader)
		// build a check map
		proposalMap := make(map[uint64]bool)
		for _, uuid := range proposal {
			proposalMap[uuid] = true
		}
		if scores != nil && proposal != nil {
			p.View = make([]uint64, 0)
			for uuid, score := range scores {
				if score > 0.65 || (score >= 0.16 && proposalMap[uuid]) {
					p.View = append(p.View, uuid)
				}
			}
		}
	}
	fmt.Printf("%s finishing RVR protocol, final View length: %d\n", p.MyId.Address, len(p.View))
	p.Finished = true
	p.FinishTime = time.Now()
}

func StartNode(controlAddress string, exitSignal chan bool) {
	p := ProtocolState{}
	p.ControlAddress = controlAddress
	p.ExitSignal = exitSignal
	go p.GetReady()
}
