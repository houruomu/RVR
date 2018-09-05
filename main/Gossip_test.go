package main

import (
	"testing"
	"RVR/message"
)

// TODO: test gossip function
func TestGossip(t *testing.T) {
	TEST_SIZE := 10
	syncLock := make(chan bool)
	peers := make([] ProtocolState, TEST_SIZE)
	for i, _ := range peers {
		go func(i int) {
			peers[i].init()
			peers[i].View = []uint64{uint64(i)}
			syncLock <- true
		}(i)
	}

	for i := 0; i < TEST_SIZE; i++ {
		<-syncLock
	}

	for i, _ := range peers {
		go func(i int) {
			for j, _ := range peers {
				peers[i].addToInitView(peers[j].MyId)
			}
			syncLock <- true
		}(i)
	}

	for i := 0; i < TEST_SIZE; i++ {
		<-syncLock
	}

	electionResult := make([]message.Identity, TEST_SIZE)
	proposalResult := make([][]uint64, TEST_SIZE)

	for i, _ := range peers {
		go func(i int) {
			electionResult[i] = DoElection(&peers[i], 1);
			proposalResult[i] = Gossip(&peers[i], &electionResult[i])
			syncLock <- true
		}(i)
	}

	for i := 0; i < TEST_SIZE; i++ {
		<-syncLock
	}

	print("the election results are:\n")
	for _, id := range electionResult {
		if (id.Public_key == nil) {
			print("nil")
		} else {
			print(id.GetUUID())
		}
		print("\n");
	}


	print("the proposal numbers are: ")
	for _, prop := range proposalResult{
		if prop != nil{
			print(prop[0])
			print("\t")
		}
	}
	print("\n");


	print("the finishing rounds are:")
	for _, p := range peers {
		print(p.Round)
		print("\t")
	}
	print("\n")



}
