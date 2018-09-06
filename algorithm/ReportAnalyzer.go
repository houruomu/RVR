package algorithm

import (
	"math"
	"sort"
	"time"
)

type Data struct{
	states []ProtocolState
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