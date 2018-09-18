package algorithm

import (
	"RVR/message"
	"errors"
	"golang.org/x/crypto/sha3"
	"crypto/rand"
	"bytes"
	"math/big"
)

type PuzzleMerkleTreeNode struct {
	lChild   *PuzzleMerkleTreeNode
	rChild   *PuzzleMerkleTreeNode
	parent   *PuzzleMerkleTreeNode
	nodeHash []byte
}

type PuzzleMerkleTree struct {
	leaveList  []PuzzleMerkleTreeNode
	idIndexMap map[uint64]int
	root       *PuzzleMerkleTreeNode
	inited     bool
}

func (node *PuzzleMerkleTreeNode) computeHash() {
	if (node.nodeHash != nil) {
		return
	}
	lHash := node.lChild.nodeHash
	var rHash []byte = nil
	if node.rChild != nil {
		rHash = node.rChild.nodeHash
	}
	hash := sha3.Sum224(append(lHash, rHash...))
	node.nodeHash = hash[:]
}

func (puz *PuzzleMerkleTree) init() {
	puz.leaveList = make([]PuzzleMerkleTreeNode, 0)
	puz.idIndexMap = make(map[uint64]int, 0)
	puz.inited = false
}

func (puz *PuzzleMerkleTree) addNonce(id message.Identity, nonce []byte) error {
	_, ok := puz.idIndexMap[id.GetUUID()]
	if ok {
		return errors.New("the id has a puzzle already")
	}
	puz.idIndexMap[id.GetUUID()] = len(puz.leaveList)
	puz.leaveList = append(puz.leaveList, PuzzleMerkleTreeNode{nil, nil, nil, nonce})
	puz.inited = false
	return nil
}

func (puz *PuzzleMerkleTree) formTree() {
	if (len(puz.leaveList) == 0) {
		return
	}
	var inProcessTreeNodes = puz.leaveList
	var tempTreeNodes = make([]PuzzleMerkleTreeNode, (len(puz.leaveList)+1)/2)
	for len(inProcessTreeNodes) > 1 {
		for i, _ := range inProcessTreeNodes {
			inProcessTreeNodes[i].computeHash()
			inProcessTreeNodes[i].parent = &tempTreeNodes[i/2]
			if i%2 == 0 {
				inProcessTreeNodes[i].parent.lChild = &inProcessTreeNodes[i]
			} else {
				inProcessTreeNodes[i].parent.rChild = &inProcessTreeNodes[i]
			}
		}
		inProcessTreeNodes = tempTreeNodes
		tempTreeNodes = make([]PuzzleMerkleTreeNode, (len(inProcessTreeNodes)+1)/2)
	}
	puz.root = &inProcessTreeNodes[0]
	puz.root.computeHash()
	puz.inited = true
}

func (puz *PuzzleMerkleTree) getProof(id message.Identity) ([][]byte, []bool, error) {
	// returns the solution and the index of the nonce
	index, ok := puz.idIndexMap[id.GetUUID()];
	if !ok {
		return nil, nil, errors.New("id's nonce not in")
	}
	if !puz.inited {
		puz.formTree()
	}

	order := make([]bool, 0)
	sol := make([][]byte, 1)
	currentNode := &puz.leaveList[index]
	sol[0] = currentNode.nodeHash
	for currentNode != puz.root {
		if (currentNode.parent.lChild == currentNode) {
			// is left child
			if (currentNode.parent.rChild == nil) {
				sol = append(sol, nil)
			} else {
				sol = append(sol, currentNode.parent.rChild.nodeHash)
			}
			order = append(order, true)
		} else {
			// is right child
			if (currentNode.parent.lChild == nil) {
				sol = append(sol, nil)
			} else {
				sol = append(sol, currentNode.parent.lChild.nodeHash)
			}
			order = append(order, false)
		}
		currentNode = currentNode.parent
	}
	return sol, order, nil
}

func EvalSol(sol [][]byte, order []bool) []byte {
	if len(sol) == 0 {
		return nil
	}
	if len(sol) == 1 {
		return sol[0]
	}
	var head [28]byte
	if order[0] {
		head = sha3.Sum224(append(sol[0], sol[1]...))
	} else {
		head = sha3.Sum224(append(sol[1], sol[0]...))
	}
	return EvalSol(append([][]byte{head[:]}, sol[2:]...), order[1:])

}

type ElectionState struct {
	parentProtocol *ProtocolState
	myNonce        []byte
	difficulty     float64 // proportion of hash to maxHash
	m              int // number of hashes per rounds
}

func DoElection(protocol *ProtocolState, m int) message.Identity {
	es := ElectionState{protocol, nil, 0, m}
	return es.DoElection()
}

func (state *ElectionState) DoElection() message.Identity {
	p := state.parentProtocol
	<- p.ticker
	p.lock.Lock()
	p.Round++
	p.lock.Unlock()

	var leader message.Identity

	// compute the difficulty

	state.difficulty = 1.0/float64(len(p.initView) * 6 * state.m * (p.offset + p.l))

	// line1: generate challenge
	state.myNonce = make([]byte, 32)
	rand.Read(state.myNonce)
	// line2: send challenge to initview for l rounds
	msg := new(message.Message)
	msg.Nonce = state.myNonce
	msg.Type = "Election Challenge"
	for i := 0; i < p.l; i++ {
		<-p.ticker

		p.lock.Lock()
		p.Round++
		msg.Round = p.Round
		p.lock.Unlock()

		msg.Sign(p.privateKey)

		for _, id := range p.initView {
			p.sendMsgToPeerAsync(*msg, id.Address)
		}
	}

	// line3\4: receive challenge from initview for offset rounds, and form a merkle tree

	mTree := new(PuzzleMerkleTree)

	mTree.init()
	mTree.addNonce(p.MyId, state.myNonce)

	for i := 0; i < p.offset; i++ {
		<-p.ticker
		p.lock.Lock()
		p.Round++
		p.lock.Unlock()

	}

	p.lock.Lock()
	for _, m := range p.inQueue {
		if _, ok := p.idToAddrMap[m.Sender.GetUUID()]; ok {
			if m.Nonce != nil {
				mTree.addNonce(m.Sender, m.Nonce)
			}
		} else {
			// message not from initview, abort
		}
	}
	p.inQueue = make([]message.Message, 0)
	p.lock.Unlock()

	mTree.formTree()

	// line 5: try to solve it for some rounds
	stopCall := make(chan bool)
	defer close(stopCall)
	var header []byte
	ifSolved := false
	go func() {
		if (len(mTree.leaveList) == 0) {
			return
		}
		solDifficulty := state.difficulty / (1.0 + p.f)
		header = make([]byte, 32)
		data := mTree.root.nodeHash
		for i := 0; i < state.m * 6 * (p.offset + p.l) && !ifSolved; i++ {
			select {
			case <-stopCall:
				return
			default:
				rand.Read(header)
				if evalHashWithDifficulty(header, data, solDifficulty) {
					leader = p.MyId
					ifSolved = true
				}else{

				}
			}
		}
		<-stopCall
	}()
	for i := 0; i < 6*(p.offset+p.l); i++ {
		<-p.ticker
		p.lock.Lock()
		p.Round++
		p.lock.Unlock()
	}
	stopCall <- true

	if ifSolved {
		// disseminate the solution for l rounds
		for i := 0; i < p.l; i++ {
			<-p.ticker
			p.lock.Lock()
			p.Round++
			p.lock.Unlock()

			p.lock.RLock()
			for _, id := range p.initView {
				msg := new(message.Message)
				var err error
				msg.Proof, msg.Order, err = mTree.getProof(id)
				if err != nil {
					// this guy's challenge is not received
					continue
				}
				msg.Nonce = header
				msg.Round = p.Round
				msg.Type = "Election Solution"
				msg.Sign(p.privateKey)
				p.sendMsgToPeerAsync(*msg, id.Address)
			}
			p.lock.RUnlock()
		}
	} else {
		// wait for l rounds
		for i := 0; i < p.l; i++ {
			<-p.ticker
			p.lock.Lock()
			p.Round++
			p.lock.Unlock()
		}
	}

	// line 10: receive solutions for offset rounds
	for i := 0; i < p.offset; i++ {
		<-p.ticker
		p.lock.Lock()
		p.Round++
		p.lock.Unlock()
	}
	// line 11-15: return the leader
	p.lock.Lock()
	for _, m := range p.inQueue {
		if _, ok := p.idToAddrMap[m.Sender.GetUUID()]; ok {
			if m.Proof == nil {
				continue
			} // not for this purpose
			if !bytes.Equal(m.Proof[0], state.myNonce) {
				continue
			} // abort if not for me
			treeHash := EvalSol(m.Proof, m.Order)
			if evalHashWithDifficulty(m.Nonce, treeHash, state.difficulty) {
				leader = m.Sender
			}else{
				//
			}
		} else {
			// message not from initview, ignore
		}
	}
	p.inQueue = make([]message.Message, 0)
	p.lock.Unlock()
	return leader
}

func evalHashWithDifficulty(header []byte, data []byte, difficulty float64) bool {
	if header == nil || data == nil{
		return false
	}
	digest := make([]byte, 32)
	hash := sha3.NewShake256()
	hash.Write(header)
	hash.Write(data)
	hash.Read(digest)
	var strength big.Int
	strength.SetBytes(digest)
	var MAX_HASH big.Int
	MAX_HASH.Exp(big.NewInt(2), big.NewInt(256), nil)

	relativeStrength := big.NewRat(1, 1).SetFrac(&strength, &MAX_HASH)
	return relativeStrength.Cmp(big.NewRat(1,1).SetFloat64(difficulty)) == -1 // strength < difficulty
}
