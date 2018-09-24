package algorithm

import (
	"fmt"
	"strings"
	"time"
)

type SpawnerState struct {
	ControlAddress string
	ExitSignal     chan bool
}

var SPAWNER_LIST  = []string {"xcna0.comp.nus.edu.sg:9697",
"xcna1.comp.nus.edu.sg:9697",
"xcna2.comp.nus.edu.sg:9697",
"xcna3.comp.nus.edu.sg:9697",
"xcna4.comp.nus.edu.sg:9697",
"xcna5.comp.nus.edu.sg:9697",
"xcna6.comp.nus.edu.sg:9697",
"xcna7.comp.nus.edu.sg:9697",
"xcna8.comp.nus.edu.sg:9697",
"xcna9.comp.nus.edu.sg:9697",
"xcna10.comp.nus.edu.sg:9697",
"xcna11.comp.nus.edu.sg:9697",
"xcna12.comp.nus.edu.sg:9697",
"xcna13.comp.nus.edu.sg:9697",
"xcna14.comp.nus.edu.sg:9697",
"xcna15.comp.nus.edu.sg:9697",
"xgpa0.comp.nus.edu.sg:9697",
"xgpa1.comp.nus.edu.sg:9697",
"xgpa2.comp.nus.edu.sg:9697",
"xgpa3.comp.nus.edu.sg:9697",
"xgpa4.comp.nus.edu.sg:9697",
"xcnb0.comp.nus.edu.sg:9697",
"xcnb1.comp.nus.edu.sg:9697",
"xcnb2.comp.nus.edu.sg:9697",
"xcnb3.comp.nus.edu.sg:9697",
"xcnb4.comp.nus.edu.sg:9697",
"xcnb5.comp.nus.edu.sg:9697",
"xcnb6.comp.nus.edu.sg:9697",
"xcnb7.comp.nus.edu.sg:9697",
"xcnb8.comp.nus.edu.sg:9697",
"xcnb9.comp.nus.edu.sg:9697",
"xcnb10.comp.nus.edu.sg:9697",
"xcnb11.comp.nus.edu.sg:9697",
"xcnb12.comp.nus.edu.sg:9697",
"xcnb13.comp.nus.edu.sg:9697",
"xcnb14.comp.nus.edu.sg:9697",
"xcnb15.comp.nus.edu.sg:9697",
"xcnb16.comp.nus.edu.sg:9697",
"xcnb17.comp.nus.edu.sg:9697",
"xcnb18.comp.nus.edu.sg:9697",
"xcnb19.comp.nus.edu.sg:9697",
"xcnc0.comp.nus.edu.sg:9697",
"xcnc1.comp.nus.edu.sg:9697",
"xcnc2.comp.nus.edu.sg:9697",
"xcnc3.comp.nus.edu.sg:9697",
"xcnc4.comp.nus.edu.sg:9697",
"xcnc5.comp.nus.edu.sg:9697",
"xcnc6.comp.nus.edu.sg:9697",
"xcnc7.comp.nus.edu.sg:9697",
"xcnc8.comp.nus.edu.sg:9697",
"xcnc9.comp.nus.edu.sg:9697",
"xcnc10.comp.nus.edu.sg:9697",
"xcnc11.comp.nus.edu.sg:9697",
"xcnc12.comp.nus.edu.sg:9697",
"xcnc13.comp.nus.edu.sg:9697",
"xcnc14.comp.nus.edu.sg:9697",
"xcnc15.comp.nus.edu.sg:9697",
"xcnc16.comp.nus.edu.sg:9697",
"xcnc17.comp.nus.edu.sg:9697",
"xcnc18.comp.nus.edu.sg:9697",
"xcnc19.comp.nus.edu.sg:9697",
"xcnc20.comp.nus.edu.sg:9697",
"xcnc21.comp.nus.edu.sg:9697",
"xcnc22.comp.nus.edu.sg:9697",
"xcnc23.comp.nus.edu.sg:9697",
"xcnc24.comp.nus.edu.sg:9697",
"xcnc25.comp.nus.edu.sg:9697",
"xcnc26.comp.nus.edu.sg:9697",
"xcnc27.comp.nus.edu.sg:9697",
"xcnc28.comp.nus.edu.sg:9697",
"xcnc29.comp.nus.edu.sg:9697",
"xcnc30.comp.nus.edu.sg:9697",
"xcnc31.comp.nus.edu.sg:9697",
"xcnc32.comp.nus.edu.sg:9697",
"xcnc33.comp.nus.edu.sg:9697",
"xcnc34.comp.nus.edu.sg:9697",
"xcnc35.comp.nus.edu.sg:9697",
"xcnc36.comp.nus.edu.sg:9697",
"xcnc37.comp.nus.edu.sg:9697",
"xcnc38.comp.nus.edu.sg:9697",
"xcnc39.comp.nus.edu.sg:9697",
"xcnc40.comp.nus.edu.sg:9697",
"xcnc41.comp.nus.edu.sg:9697",
"xcnc42.comp.nus.edu.sg:9697",
"xcnc43.comp.nus.edu.sg:9697",
"xcnc44.comp.nus.edu.sg:9697",
"xcnc45.comp.nus.edu.sg:9697",
"xcnc46.comp.nus.edu.sg:9697",
"xcnc47.comp.nus.edu.sg:9697",
"xcnc48.comp.nus.edu.sg:9697",
"xcnc49.comp.nus.edu.sg:9697",
"xcnd0.comp.nus.edu.sg:9697",
"xcnd1.comp.nus.edu.sg:9697",
"xcnd2.comp.nus.edu.sg:9697",
"xcnd3.comp.nus.edu.sg:9697",
"xcnd4.comp.nus.edu.sg:9697",
"xcnd5.comp.nus.edu.sg:9697",
"xcnd6.comp.nus.edu.sg:9697",
"xcnd7.comp.nus.edu.sg:9697",
"xcnd8.comp.nus.edu.sg:9697",
"xcnd9.comp.nus.edu.sg:9697",
"xcnd10.comp.nus.edu.sg:9697",
"xcnd11.comp.nus.edu.sg:9697",
"xcnd12.comp.nus.edu.sg:9697",
"xcnd13.comp.nus.edu.sg:9697",
"xcnd14.comp.nus.edu.sg:9697",
"xcnd15.comp.nus.edu.sg:9697",
"xcnd16.comp.nus.edu.sg:9697",
"xcnd17.comp.nus.edu.sg:9697",
"xcnd18.comp.nus.edu.sg:9697",
"xcnd19.comp.nus.edu.sg:9697",
"xcnd20.comp.nus.edu.sg:9697",
"xcnd21.comp.nus.edu.sg:9697",
"xcnd22.comp.nus.edu.sg:9697",
"xcnd23.comp.nus.edu.sg:9697",
"xcnd24.comp.nus.edu.sg:9697",
"xcnd25.comp.nus.edu.sg:9697",
"xcnd26.comp.nus.edu.sg:9697",
"xcnd27.comp.nus.edu.sg:9697",
"xcnd28.comp.nus.edu.sg:9697",
"xcnd29.comp.nus.edu.sg:9697",
"xcnd30.comp.nus.edu.sg:9697",
"xcnd31.comp.nus.edu.sg:9697",
"xcnd32.comp.nus.edu.sg:9697",
"xcnd33.comp.nus.edu.sg:9697",
"xcnd34.comp.nus.edu.sg:9697",
"xcnd35.comp.nus.edu.sg:9697",
"xcnd36.comp.nus.edu.sg:9697",
"xcnd37.comp.nus.edu.sg:9697",
"xcnd38.comp.nus.edu.sg:9697",
"xcnd39.comp.nus.edu.sg:9697",
"xcnd40.comp.nus.edu.sg:9697",
"xcnd41.comp.nus.edu.sg:9697",
"xcnd42.comp.nus.edu.sg:9697",
"xcnd43.comp.nus.edu.sg:9697",
"xcnd44.comp.nus.edu.sg:9697",
"xcnd45.comp.nus.edu.sg:9697",
"xcnd46.comp.nus.edu.sg:9697",
"xcnd47.comp.nus.edu.sg:9697",
"xcnd48.comp.nus.edu.sg:9697",
"xcnd49.comp.nus.edu.sg:9697",
"xcnd50.comp.nus.edu.sg:9697",
"xcnd51.comp.nus.edu.sg:9697",
"xcnd52.comp.nus.edu.sg:9697",
"xcnd53.comp.nus.edu.sg:9697",
"xcnd54.comp.nus.edu.sg:9697",
"xcnd55.comp.nus.edu.sg:9697",
"xcnd56.comp.nus.edu.sg:9697",
"xcnd57.comp.nus.edu.sg:9697",
"xcnd58.comp.nus.edu.sg:9697",
"xcnd59.comp.nus.edu.sg:9697",
"xgpb0.comp.nus.edu.sg:9697",
"xgpb1.comp.nus.edu.sg:9697",
"xgpb2.comp.nus.edu.sg:9697",
"xgpc0.comp.nus.edu.sg:9697",
"xgpc1.comp.nus.edu.sg:9697",
"xgpc2.comp.nus.edu.sg:9697",
"xgpc3.comp.nus.edu.sg:9697",
"xgpc4.comp.nus.edu.sg:9697",
"xgpc5.comp.nus.edu.sg:9697",
"xgpc6.comp.nus.edu.sg:9697",
"xgpc7.comp.nus.edu.sg:9697",
"xgpc8.comp.nus.edu.sg:9697",
"xgpc9.comp.nus.edu.sg:9697",
"xgpd0.comp.nus.edu.sg:9697",
"xgpd1.comp.nus.edu.sg:9697",
"xgpd2.comp.nus.edu.sg:9697",
"xgpd3.comp.nus.edu.sg:9697",
"xgpd4.comp.nus.edu.sg:9697",
"xgpd5.comp.nus.edu.sg:9697",
"xgpd6.comp.nus.edu.sg:9697",
"xgpd7.comp.nus.edu.sg:9697",
"xgpd8.comp.nus.edu.sg:9697",
"xgpd9.comp.nus.edu.sg:9697",
}

func (s *SpawnerState) Spawn(count int, rtv *int) error {
	for i := 0; i < count; i++ {
		go func() {
			_exitSignal := make(chan bool, 5)
			node := StartNode(s.ControlAddress, _exitSignal)
			for{
				time.Sleep(20 * time.Second)
				select{
				case <-_exitSignal:
					_exitSignal <- true
					fmt.Printf("%s: node exiting\n", node.MyId.Address)
					return
				}
			}
		}()
	}
	return nil
}

func (s *SpawnerState) Exit(count int, rtv *int) error {
	go func(term chan bool) { time.Sleep(100 * time.Microsecond); term <- true }(s.ExitSignal)
	return nil
}

func (s *SpawnerState) Start() {
	myIP := GetOutboundAddr()
	portString := ListenRPC(":9697", s, s.ExitSignal)
	myPort := portString[strings.LastIndex(portString, ":"):]
	addr := myIP + myPort
	// report to controller

	RpcCall(s.ControlAddress, "ControllerState.RegisterServer", addr, nil, time.Second)
	fmt.Printf("Spawner Ready: %s\n", addr)

}

func StartSpawner(controlAddr string, exitSignal chan bool) {
	server := SpawnerState{ControlAddress: controlAddr, ExitSignal: exitSignal}
	server.Start()
}

func (s *SpawnerState) BlackHole(msg []byte, rtv *int) error {
	// this is a blackhole function for measuring ping value
	for i, _ := range msg {
		if msg[i] == 0 {
			return nil
		}
	}
	return nil
}
