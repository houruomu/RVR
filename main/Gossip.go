package main

import (
	"RVR/message"
	"bytes"
)

func Gossip(p *ProtocolState, leader *message.Identity) []uint64{

	<- p.ticker
	p.lock.Lock()
	p.round++
	p.lock.Unlock()


	var proposal []uint64
	if leader.Public_key == nil{
		for i := 0; i < p.x; i++{
			<- p.ticker
			p.lock.Lock()
			p.round++
			p.lock.Unlock()
		}
		return proposal
	}
	var msg message.Message
	if bytes.Equal(leader.Public_key, p.myId.Public_key){
		proposal = p.view
		msg.Round = p.round
		msg.View = proposal
		msg.Sender = p.myId
		msg.Type = "Gossip Message"
		msg.Sign(p.privateKey)
	}else {proposal = nil}

	for i := 0; i < p.x; i++{
		// notice to facilitate gossip, we only increase the round at the end
		<- p.ticker
		// p.round++ (defered to the end of this function)
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
			// deliver to initview
			for _, id := range p.initView {
				p.sendMsgToPeerAsync(msg, id.Address)
			}
		}
	}

	for i := 0; i < p.x; i++{
		// notice to facilitate gossip, we only increase the round at the end
		p.lock.Lock()
		p.round++
		p.lock.Unlock()
	}

	return proposal
}