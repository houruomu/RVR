package main

import (
	"testing"
	"RVR/message"
	"fmt"
	"crypto/rsa"
	"crypto/rand"
	"crypto/x509"
	"bytes"
)

type puzzleStruct struct {
	id    message.Identity
	nonce []byte
} // this class is for testing

func TestMerkleTree_formtree_testTree(t *testing.T) {
	testSize := 20

	puz := PuzzleMerkleTree{}
	puz.init()

	tester := make([]puzzleStruct, testSize)
	for i, x := range tester {
		x.id.Address = fmt.Sprintf("tester %d", i)
		privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
		x.id.Public_key = x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)
		x.nonce = make([]byte, 28)
		rand.Read(x.nonce)
		tester[i] = x
		puz.addNonce(x.id, x.nonce)
	}

	puz.formTree()

	for _, x := range tester {
		sol, order, _ := puz.getProof(x.id)
		if !bytes.Equal(sol[0], x.nonce) {
			t.Error("Wrong Position in Proof")
		}
		if !bytes.Equal(EvalSol(sol, order), puz.root.nodeHash) {
			t.Error("Not evaluating to the right answer")
		}
	}

}

func TestDoElection(t *testing.T) {
	TEST_SIZE := 10
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
			for j, _ := range peers{
				peers[i].addToInitView(peers[j].MyId)
			}
			syncLock <- true
		}(i)
	}

	for i := 0; i < TEST_SIZE; i++ {
		<-syncLock
	}

	electionResult := make([]message.Identity, TEST_SIZE)

	for i, _ := range peers {
		go func(i int) { electionResult[i] = DoElection(&peers[i], 1); syncLock <- true }(i)
	}

	for i := 0; i < TEST_SIZE; i++ {
		<-syncLock
	}

	for _, p := range peers{
		print(p.Round)
		print("\n")
	}

	for _, id := range electionResult{
		print("\n") ;
		if(id.Public_key == nil){
			print("nil")
		}else{
			print(id.GetUUID())
		}
	}
	print("\n")

}
