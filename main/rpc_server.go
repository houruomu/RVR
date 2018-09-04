package main

import (
	"RVR/message"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"flag"
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
var CONTROL_ADDRESS = "172.24.200.200:9696"

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
	round       int
	inQueue     []message.Message
	view        []uint64
	lock        sync.RWMutex
	ticker      <-chan time.Time
	idToAddrMap map[uint64]string // use to check whether in initview
	myId        message.Identity
	finished    bool
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

func GetOutboundAddr() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

func main(){
	mode := flag.String("mode", "node", "choose a value between node/controller")
	flag.Parse()
	switch *mode{
	case "node":
		p := ProtocolState{}
		p.getReady()

	case "controller":
		c := ControllerState{defaultSetupParams, make([]message.Identity, 0), ""}
		c.startListen()

	default:
		log.Fatalf("Unsupported mode: %s\n Try:node/controller\n", mode)
		return
	}
	time.Sleep(1000000000 * time.Second) // run forever
}

func (p *ProtocolState) SendInMsg(msg message.Message, rtv *int) error {
	p.lock.RLock()
	if (msg.Round < p.round-p.offset) {
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
	p.round = 1
	p.inQueue = make([]message.Message, 0)
	p.initView = make([]message.Identity, 0)
	p.view = make([]uint64, 0)
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
	p.myId = message.Identity{myIP + myPort, x509.MarshalPKCS1PublicKey(&p.privateKey.PublicKey)}
	p.initView = append(p.initView, p.myId)
	p.idToAddrMap[p.myId.GetUUID()] = p.myId.Address
	for _, id := range p.initView {
		p.idToAddrMap[id.GetUUID()] = id.Address
	}

	log.Printf("RPC Server started, Listening on %s", p.myId.Address)
} // this function starts the protocol at once

func (p *ProtocolState) getReady() { // this function setup the server in a waiting-for-instruct phase
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
	p.myId = message.Identity{myIP + myPort, x509.MarshalPKCS1PublicKey(&p.privateKey.PublicKey)}
	// report to controller
	client, err := rpc.Dial("tcp", CONTROL_ADDRESS)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer client.Close()
	fmt.Printf("Node ready to receive instructions, address: %s\n", p.myId.Address)
	client.Go("ControllerState.Register", p.myId, nil, nil)
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
	p.round = 1
	p.inQueue = make([]message.Message, 0)
	p.view = make([]uint64, 0)
	p.idToAddrMap = make(map[uint64]string)

	for i, _ := range p.initView{
		p.idToAddrMap[p.initView[i].GetUUID()] = p.initView[i].Address
	}

	fmt.Printf("Node setup done, initview length: %d\n", len(p.initView))
	return nil
}

func (p *ProtocolState) SetView(view []uint64, rtv *int) error {
	p.view = view
	return nil
}

func (p *ProtocolState) Start(command int, rtv *int) error {
	// this function starts the algorithm
	// start the ticker
	p.ticker = time.Tick(p.roundDuration)
	// invoke view Reconciliation (asynchrously)
	go p.viewReconciliation()
	return nil
}

func (p *ProtocolState) addToInitView(id message.Identity) {
	if _, ok := p.idToAddrMap[id.GetUUID()]; !ok {
		p.initView = append(p.initView, id)
		p.idToAddrMap[id.GetUUID()] = id.Address
		nEstimate := float64(len(p.initView))
		p.x = int(math.Ceil(math.Log(nEstimate)/math.Log(math.Log(nEstimate))+4.0))*p.l + p.offset // TODO: when N is small ,this value could go negative
	}
}

func (p *ProtocolState) sendMsgToPeerAsync(m message.Message, addr string) {
	// TODO: implement a connection pool
	go func() {
		client, err := rpc.Dial("tcp", addr)
		if err != nil {
			log.Fatal("dialing:", err.Error())
			return
		}
		defer client.Close()
		client.Go("ProtocolState.SendInMsg", m, nil, nil)
		// fmt.Printf("%s is sending msg %s to %s\n", p.myId.Address, m.Type, addr )
	}()
}

func (p *ProtocolState) updateWithPeers(peers []string, maxRound int) {
	// every round advertise one of my peer to all my peers
	// succeed if heard from every one
	for totalRound := maxRound; totalRound > 0; totalRound-- {
		<-p.ticker // round counter
		p.lock.Lock()
		p.round++
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
		m.Sender = p.myId
		// since I have changed the way it works, we need to broadcast a random guy to all peers
		m.Sender = p.initView[rand2.Int()%len(p.initView)]

		p.lock.RLock()
		m.Round = p.round
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
	fmt.Printf("%s starting RVR protocol, initial view length: %d\n", p.myId.Address, len(p.view))
	repetity := int(6.0*math.Log(2/p.delta) + 1)
	// repetity /= 32
	// TODO: init view
	for i := 0; i < repetity; i++ {
		leader := DoElection(p, 1)
		scores := Sample(p)
		if bytes.Equal(leader.Public_key, p.myId.Public_key) {
			if scores != nil {
				p.view = make([]uint64, 0)
				for uuid, score := range scores {
					if score > 0.4 {
						p.view = append(p.view, uuid)
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
			p.view = make([]uint64, 0)
			for uuid, score := range scores {
				if score > 0.65 || (score >= 0.16 && proposalMap[uuid]) {
					p.view = append(p.view, uuid)
				}
			}
		}
	}
	fmt.Printf("%s finishing RVR protocol, final view length: %d\n", p.myId.Address, len(p.view))
	p.finished = true
}
