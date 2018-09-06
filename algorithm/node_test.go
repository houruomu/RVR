package algorithm

import (
	"testing"
	"net/rpc"
	"log"
	"net"
	"crypto/rsa"
	"crypto/rand"
	"RVR/message"
	"time"
	"crypto/x509"
	rand2 "math/rand"
	"fmt"
)

func TestProtocolState_SendInMsg(t *testing.T) {
	server := new(ProtocolState)
	server.Round = 1
	rpc.Register(server)
	l, e := net.Listen("tcp", ":18374")
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
			go rpc.ServeConn(conn)
		}
	}()

	client, err := rpc.Dial("tcp", "localhost:18374")
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer client.Close()

	// prepare msg
	msg := new(message.Message)
	msg.Round = 50
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	msg.View = make([]uint64, 1)
	identity := message.Identity{"abcd", x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)}
	msg.View[0] = identity.GetUUID()
	msg.Sign(privateKey)

	// Async call
	client.Go("ProtocolState.SendInMsg", msg, nil, nil)
	time.Sleep(300 * time.Millisecond)

	// Sync call
	err = client.Call("ProtocolState.SendInMsg", msg, nil)
	if err != nil {
		t.Error(err.Error())
	}

	if len(server.inQueue) != 2 {
		t.Error("error registering message")
		print(len(server.inQueue))
	}

	server.Round = 70
	// Sync Call
	err = client.Call("ProtocolState.SendInMsg", msg, nil)
	if err == nil {
		t.Error("accepting expired msg")
	}

	msg.Round = 90
	err = client.Call("ProtocolState.SendInMsg", msg, nil)
	if err == nil {
		t.Error("accepting wrongly signed msg")
	}

	print(len(server.inQueue[0].View))

}

func TestProtocolState_test_init_Seq(t *testing.T) {
	p1 := new(ProtocolState)
	p2 := new(ProtocolState)
	p3 := new(ProtocolState)
	p4 := new(ProtocolState)
	p5 := new(ProtocolState)
	p6 := new(ProtocolState)
	p7 := new(ProtocolState)
	p8 := new(ProtocolState)
	p9 := new(ProtocolState)
	p10 := new(ProtocolState)

	go p1.init()
	go p2.init()
	go p3.init()
	go p4.init()
	go p5.init()
	go p6.init()
	go p7.init()
	go p8.init()
	go p9.init()
	go p10.init()

	time.Sleep(1000 * time.Millisecond)

	log.Print("initialization sequence completed")
	maxRound := 20
	// perround time is 100ms
	go p1.updateWithPeers([]string{p2.MyId.Address, p6.MyId.Address}, maxRound)
	go p2.updateWithPeers([]string{p1.MyId.Address, p3.MyId.Address}, maxRound)
	go p3.updateWithPeers([]string{p2.MyId.Address, p5.MyId.Address, p6.MyId.Address}, maxRound)
	go p4.updateWithPeers([]string{p3.MyId.Address, p5.MyId.Address, p6.MyId.Address, p7.MyId.Address}, maxRound)
	go p5.updateWithPeers([]string{p4.MyId.Address, p3.MyId.Address}, maxRound)
	go p6.updateWithPeers([]string{p1.MyId.Address, p3.MyId.Address, p4.MyId.Address}, maxRound)
	go p7.updateWithPeers([]string{p4.MyId.Address, p8.MyId.Address}, maxRound)
	go p8.updateWithPeers([]string{p7.MyId.Address, p9.MyId.Address}, maxRound)
	go p9.updateWithPeers([]string{p8.MyId.Address, p10.MyId.Address}, maxRound)
	go p10.updateWithPeers([]string{p9.MyId.Address}, maxRound)

	time.Sleep(3 * time.Second)

	if len(p1.initView) != 10 {
		t.Error("synchronization failed: not all nodes in the View")
		print(len(p1.initView))
	} else {
		t.Log("sychronization succeeded")
	}

	if (p1.Round != p2.Round || p1.Round != p3.Round || p1.Round != p4.Round || p1.Round != p5.Round || p1.Round != p6.Round || p1.Round != p7.Round || p1.Round != p8.Round) {
		t.Error("nodes went out of sync")
	} else {
		t.Log("nodes are in sync after the algo")
	}
}

type RpcDummy struct {
	difficulty int // the difficulty of rpc call
	counter    int // the number of rpc call served
}

func (r *RpcDummy) Serve(msg int, rtv *int) error {
	// ignore the difficulty for now
	*rtv = msg + 1
	r.counter++
	return nil
}

func Test_rpc_load(t *testing.T) {
	s := new(RpcDummy)
	s.difficulty = 1000000
	handler := rpc.NewServer()
	handler.Register(s)
	l, e := net.Listen("tcp", "127.0.0.1:8082")
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
	for i := 0; i < 1000; i++ {
		go func() {
			client, err := rpc.Dial("tcp", "127.0.0.1:8082")
			if err != nil {
				return
			}
			defer client.Close()
			for i := 0; i < s.difficulty/1000; i++ {
				client.Go("RpcDummy.Serve", 1, nil, nil)
			}
		}()
	}
	time.Sleep(5000 * time.Millisecond)
	print(s.counter)
	// result: about 140 requests per millisecond
}

func Test_RVR(t *testing.T) {
	TEST_SIZE := 30
	viewDist := make(map[uint64]float64)
	// prepare the View distribution
	for i := 0 ; i < 10; i++{
		viewDist[uint64(i)] = float64(i) / 10
	}

	syncLock := make(chan bool)
	peers := make([] ProtocolState, TEST_SIZE)
	for i,_ := range peers {
		go func(i int) {
			peers[i].init()
			syncLock <- true
		}(i)
	}

	for i := 0; i < TEST_SIZE; i++ {
		<-syncLock
	}

	for i, _ := range peers {
		go func(i int) {
			view := make([]uint64, 0)
			for id, prob := range viewDist{
				if rand2.Float64() < prob{
					view = append(view, uint64(id))
				}
			}
			peers[i].View = view
			for j, _ := range peers{
				peers[i].addToInitView(peers[j].MyId)
			}
			syncLock <- true
		}(i)
	}

	for i := 0; i < TEST_SIZE; i++ {
		<-syncLock
	}
	for i, _ := range peers {
		fmt.Printf("peer[%d]:", i )
		for _, id := range peers[i].View {
			fmt.Printf("%d\t",id)
		}
		print("\n")
	}

	for i, _ := range peers {
		go func(i int) { peers[i].viewReconciliation(); syncLock <- true }(i)
	}

	for i := 0; i < TEST_SIZE; i++ {
		<-syncLock
	}
	print("rounds: ")
	for _, p := range peers{
		print(p.Round)
		print("\t")
	}
	print("\n")

	for i, _ := range peers {
		fmt.Printf("peer[%d]:", i )
		for _, id := range peers[i].View {
			fmt.Printf("%d\t",id)
		}
		print("\n")
	}
	print("\n")
}

