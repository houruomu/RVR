package algorithm

import (
	"RVR/message"
	"bytes"
	"fmt"
	"math"
	"math/rand"
)

func Gossip(p *ProtocolState, leader *message.Identity) []uint64{

	<- p.ticker
	p.lock.Lock()
	p.Round++
	p.lock.Unlock()


	var proposal []uint64
	if leader.Public_key == nil{
		fmt.Printf("Leader Election failed, round %d.\n", p.Round)
		for i := 0; i < p.x; i++{
			<- p.ticker
			p.lock.Lock()
			p.Round++
			p.lock.Unlock()
		}
		return proposal
	}
	var msg message.Message
	if bytes.Equal(leader.Public_key, p.MyId.Public_key){
		proposal = p.View
		msg.Round = p.Round
		msg.View = proposal
		msg.Sender = p.MyId
		msg.Type = "Gossip Message"
		msg.Sign(p.privateKey)
	}else {proposal = nil}

	for i := 0; i < p.x; i++{
		// notice to facilitate gossip, we only increase the Round at the end
		<- p.ticker
		// p.Round++ (defered to the end of this function)
		if proposal == nil{
			// try to receive from initview
			p.lock.Lock()
			for _, m := range p.inQueue {
				if _, ok := p.idToAddrMap[m.Sender.GetUUID()]; ok {
					if bytes.Equal(m.Sender.Public_key, leader.Public_key){
						proposal = m.View
						msg = m
						break
					}
				} else {
					// message not from initview, abort
				}
			}
			p.inQueue = make([]message.Message, 0)
			p.lock.Unlock()
		} else {
			// deliver to 8(1+f)ln|initview| / delta members in initview
			p.lock.RLock()
			gossipSize := int(math.Min(float64(len(p.initView)),math.Ceil(8 * (1+p.f) * math.Log(float64(len(p.initView))) / p.delta)))
			perm := rand.Perm(gossipSize)
			for _,i := range perm {
				p.sendMsgToPeerAsync(msg, p.initView[i].Address)
			}
			p.lock.RUnlock()
		}
	}
	p.lock.Lock()
	// notice to facilitate gossip, we only increase the Round at the end
	p.Round += p.x
	p.lock.Unlock()

	return proposal
}