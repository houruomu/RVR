package algorithm

import (
	"fmt"
	"math"
	"sort"
	"time"
)

type Data struct{
	states []ProtocolState
	setupParam ProtocolRPCSetupParams
}

func (data *Data) checkConsensus() bool{
	if len(data.states) == 0 {
		return true
	}

	viewHashMap := make(map[uint64]bool, len(data.states[0].View))
	for i, _ := range data.states[0].View{
		viewHashMap[data.states[0].View[i]] = true
	}
	for i, _ := range data.states{
		if len(data.states[i].View) != len(data.states[0].View){
			return false
		}
		for j, _ := range data.states[i].View{
			_, ok := viewHashMap[data.states[i].View[j]]
			if !ok{
				return false
			}
		}
	}
	return true
}

func (data *Data) checkFinished() bool{
	for i, _ := range data.states{
		if !data.states[i].Finished{
			return false
		}
	}
	return true
}
type PingValueReport []int
type durationSlice []time.Duration
func(d durationSlice) Len() int{
	return len(d)
}

func(d durationSlice) Swap(i,j int){
	d[i], d[j] = d[j], d[i]
}

func(d durationSlice) Less(i,j int) bool{
	return d[i] < d[j]
}

func (data *Data) time(percentile float64) time.Duration{
	if len(data.states) == 0{
		return 0;
	}
	times := make([]time.Duration, len(data.states))
	for i, _ := range data.states{
		times[i] = data.states[i].FinishTime.Sub(data.states[i].StartTime)
	}
	sort.Sort(durationSlice(times))
	return times[int(math.Floor(percentile * (float64(len(times)) - 0.51)))]
}

func (data *Data) msgCount(percentile float64) int {
	counts := make([]int, len(data.states))
	for i, _ := range data.states{
		counts[i] = data.states[i].MsgCount
	}
	sort.Sort(sort.IntSlice(counts))
	return counts[int(math.Floor(percentile * (float64(len(counts)) - 0.51)))]
}

func (data *Data) byteCount(percentile float64) int {
	counts := make([]int, len(data.states))
	for i, _ := range data.states{
		counts[i] = data.states[i].ByteCount
	}
	sort.Sort(sort.IntSlice(counts))
	return counts[int(math.Floor(percentile * (float64(len(counts)) - 0.51)))]
}

func (d *Data) Report() string{
	if len(d.states) == 0{return ""}
	report := fmt.Sprintf("%t, %t, %d, %d, %f, %f, %d, %d, %f, %d, %d, %d, %d, %d, %d\n",
		d.checkFinished(),
		d.checkConsensus(),
		d.setupParam.RoundDuration,
		d.setupParam.Offset,
		d.setupParam.F,
		d.setupParam.G,
		d.setupParam.L,
		d.setupParam.X,
		d.setupParam.Delta,
		d.time(0.5),
		d.time(0.9),
		d.msgCount(0.5),
		d.msgCount(0.9),
		d.byteCount(0.5),
		d.byteCount(0.9))
	return report
}