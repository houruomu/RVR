package algorithm

import (
	"crypto/rand"
	"math"
	"golang.org/x/crypto/sha3"
	"RVR/message"
	"bytes"
	"fmt"
	"sync"
)

func Sample(p *ProtocolState) map[uint64]float64{
	<- p.ticker
	p.lock.Lock()
	p.Round++
	p.lock.Unlock()


	received := make(map[uint64] bool)
	// line 1: Generate Nonce
	nonce := make([]byte, 32)
	rand.Read(nonce)

	// line 2: compute s_u
	sU := (1.0 / float64(len(p.initView))) * math.Max(float64(10.0 / (1-25*p.g-7.5*p.f) / (1-25*p.g-7.5*p.f)), float64(720*(1.0+p.f)*(3+5*p.f)/(1-7*p.f)/(1-7*p.f))) * math.Log(3*float64(len(p.initView))/p.delta)
	sU = math.Min(sU, 1)
	difficulty := sU
	loweredDifficulty := difficulty * (1+p.f)

	// line 3: send hash(c_u) to nodes in initview
	commit := sha3.New256().Sum(nonce)
	msg := new(message.Message)
	msg.Nonce = commit
	msg.Type = "Sample Commitment"
	sentList := make(map[string]bool)
	localLock := sync.Mutex{}
	for i := 0; i < p.l; i++ {
		<-p.ticker

		p.lock.Lock()
		p.Round++
		msg.Round = p.Round
		p.lock.Unlock()
		msg.Sender = p.MyId
		msg.Sign(p.privateKey)


		for _, id := range p.initView {
			go func(addr string){
				localLock.Lock()
				if _, ok := sentList[addr]; ok{
					localLock.Unlock()
					return
				}
				localLock.Unlock()

				err := p.sendMsgToPeerWithTrial(*msg, addr, 1)
				if err == nil{
					localLock.Lock()
					sentList[addr] = true
					localLock.Unlock()
				}
			}(id.Address)
			//p.sendMsgToPeerAsync(*msg, id.Address)
		}
	}

	// line 4: receive the commitments for offset rounds
	commitMap := make(map[uint64][]byte)
	for i := 0; i < p.offset; i++ {
		<-p.ticker
		p.lock.Lock()
		p.Round++
		p.lock.Unlock()
	}
	bufferQueue :=  make([]message.Message, 0)
	p.lock.Lock()
	for _, m := range p.inQueue {
		if m.Round > p.Round {
			bufferQueue = append(bufferQueue, m)
			continue
		}
		if _, ok := p.idToAddrMap[m.Sender.GetUUID()]; !ok {
			print("Message invalid: Not from initview\n")
			continue
		}

		if m.Type != "Sample Commitment"{
			fmt.Printf("Message invalid: Type mismatch, expecting %s, get %s\n", "Sample Commitment", m.Type )
			continue
		}
		commitMap[m.Sender.GetUUID()] = m.Nonce
	}
	p.inQueue = bufferQueue
	p.lock.Unlock()

	// line 5: send nonce to every node
	msg.Nonce = nonce
	msg.Type = "Sample Nonce"
	sentList = make(map[string] bool)
	for i := 0; i < p.l; i++ {
		<-p.ticker

		p.lock.Lock()
		p.Round++
		msg.Round = p.Round
		p.lock.Unlock()
		msg.Sender = p.MyId

		msg.Sign(p.privateKey)

		for _, id := range p.initView {
			go func(addr string){
				localLock.Lock()
				if _, ok := sentList[addr]; ok{
					localLock.Unlock()
					return
				}
				localLock.Unlock()

				err := p.sendMsgToPeerWithTrial(*msg, addr, 1)
				if err == nil{
					localLock.Lock()
					sentList[addr] = true
					localLock.Unlock()
				}
			}(id.Address)
			//p.sendMsgToPeerAsync(*msg, id.Address)
		}
	}

	// line 6: wait for offset rounds
	for i := 0; i < p.offset; i++ {
		<-p.ticker
		p.lock.Lock()
		p.Round++
		p.lock.Unlock()
	}

	// line 7-8: generate the sample based on previous results, send for l rounds
	toSend := make([]message.Identity,0)
	toSendNull := make([]message.Identity,0)
	bufferQueue =  make([]message.Message, 0)
	p.lock.Lock()
	for _, m := range p.inQueue {
		if m.Round > p.Round {
			bufferQueue = append(bufferQueue, m)
			continue
		}
		if _, ok := p.idToAddrMap[m.Sender.GetUUID()]; !ok {
			print("Message invalid: Not from initview\n")
			continue
		}

		if m.Type != "Sample Nonce"{
			fmt.Printf("Message invalid: Type mismatch, expecting %s, get %s\n", "Sample Nonce", m.Type )
			continue
		}
		if _, ok := received[m.Sender.GetUUID()]; ok{
			// print("Message invalid: Duplicate message\n")
			continue
		}

		if _, ok := commitMap[m.Sender.GetUUID()]; !ok{
			print("Message invalid: commitment not received\n")
			continue
		}

		if com := commitMap[m.Sender.GetUUID()]; !bytes.Equal(sha3.New256().Sum(m.Nonce), com){
			print("Message invalid: nonce not matching commited value\n")
			continue
		}
		received[m.Sender.GetUUID()] = true
		if evalHashWithDifficulty(m.Nonce, nonce, loweredDifficulty){
			toSend = append(toSend, m.Sender)
		}else{
			toSendNull = append(toSendNull, m.Sender)
		}

	}
	p.inQueue = bufferQueue
	p.lock.Unlock()


	msg.Nonce = nonce
	msg.View = p.View
	nilMsg := new(message.Message)
	nilMsg.Nonce = nonce
	msg.Type = "Sample View"
	nilMsg.Type = "Sample Nil Message"
	sentList = make(map[string] bool)
	for i := 0; i < p.l; i++ {
		<-p.ticker

		p.lock.Lock()
		p.Round++
		msg.Round = p.Round
		nilMsg.Round = p.Round
		p.lock.Unlock()
		msg.Sender = p.MyId
		nilMsg.Sender = p.MyId

		msg.Sign(p.privateKey)
		nilMsg.Sign(p.privateKey)

		for _, id := range toSend {
			go func(addr string){
				localLock.Lock()
				if _, ok := sentList[addr]; ok{
					localLock.Unlock()
					return
				}
				localLock.Unlock()

				err := p.sendMsgToPeerWithTrial(*msg, addr, 1)
				if err == nil{
					localLock.Lock()
					sentList[addr] = true
					localLock.Unlock()
				}
			}(id.Address)
			//p.sendMsgToPeerAsync(*msg, id.Address)
		}
		for _, id := range toSendNull {
			go func(addr string){
				localLock.Lock()
				if _, ok := sentList[addr]; ok{
					localLock.Unlock()
					return
				}
				localLock.Unlock()

				err := p.sendMsgToPeerWithTrial(*msg, addr, 1)
				if err == nil{
					localLock.Lock()
					sentList[addr] = true
					localLock.Unlock()
				}
			}(id.Address)
			//p.sendMsgToPeerAsync(*nilMsg, id.Address)
		}
	}

	// line 9: wait for offset rounds
	for i := 0; i < p.offset; i++ {
		<-p.ticker
		p.lock.Lock()
		p.Round++
		p.lock.Unlock()
	}

	// line 10-14: compute the result
	p.lock.Lock()

	score := make(map[uint64]float64)
	sampleCount := 0
	received = make(map[uint64]bool)
	bufferQueue = make([]message.Message,0)
	for _, m := range p.inQueue {
		if m.Round > p.Round {
			bufferQueue = append(bufferQueue, m)
			continue
		}
		if _, ok := p.idToAddrMap[m.Sender.GetUUID()]; !ok {
			print("Message invalid: Not from initview\n")
			continue
		}

		if !(m.Type == "Sample View"|| m.Type == "Sample Nil Message"){
			fmt.Printf("Message invalid: Type mismatch, expecting %s, get %s\n", "Sample View", m.Type )
			fmt.Printf("The message with Round %d is received in Round %d\n", m.Round, p.Round)
			continue
		}

		if _, ok := received[m.Sender.GetUUID()]; ok{
			// print("Message invalid: Duplicate message\n")
			continue
		}

		if _, ok := commitMap[m.Sender.GetUUID()]; !ok{
			print("Message invalid: commitment not received\n")
			continue
		}

		if com := commitMap[m.Sender.GetUUID()]; !bytes.Equal(sha3.New256().Sum(m.Nonce), com){
			print("Message invalid: nonce not matching commited value\n")
			continue
		}
		sampleCount++
		received[m.Sender.GetUUID()] = true
		if evalHashWithDifficulty(nonce, m.Nonce, difficulty){
			for _, id := range m.View{
				score[id] = score[id]+1
			}
		}
	}
	p.inQueue = bufferQueue
	p.lock.Unlock()
	sampleTarget := (1-4*p.g)/(1+p.f)*float64(len(p.initView))
	if float64(sampleCount) < sampleTarget{
		fmt.Printf("Sample Failed on Round: %d due to not enough Samples, target: %f, received: %d.\n", p.Round, sampleTarget, sampleCount)
		return nil
	}else{
		for i, _ := range score{
			score[i] = score[i] / float64(len(p.initView)) / sU
		}
		return score
	}
}