package algorithm

import (
	"testing"
	"math/rand"
	"fmt"
)

func TestSample(t *testing.T) {
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
				if rand.Float64() < prob{
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

	sampleResults := make([]map[uint64]float64, TEST_SIZE)

	for i, _ := range peers {
		go func(i int) { sampleResults[i] = Sample(&(peers[i])); syncLock <- true }(i)
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

	for _, scores := range sampleResults{
		print("\n") ;
		if(scores == nil){
			print("nil")
		}else{
			for id, score := range scores{
				fmt.Printf("%d: %f\n", id, score)
			}
		}
	}
	print("\n")
}
