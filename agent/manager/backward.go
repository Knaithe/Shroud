package manager

import (
	"net"
	"sync"
)

type backwardManager struct {
	mu sync.Mutex

	backwardSeqMap   map[uint64]string
	backwardMap      map[string]*backward
	BackwardMessChan chan interface{}
	SeqReady         chan bool
}

type backward struct {
	listener net.Listener
	seqChan  chan uint64

	backwardStatusMap map[uint64]*backwardStatus
}

type backwardStatus struct {
	dataChan chan []byte
}

func newBackwardManager() *backwardManager {
	manager := new(backwardManager)
	manager.backwardSeqMap = make(map[uint64]string)
	manager.backwardMap = make(map[string]*backward)
	manager.BackwardMessChan = make(chan interface{}, 5)
	manager.SeqReady = make(chan bool)
	return manager
}

func (m *backwardManager) NewBackward(rPort string, listener net.Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.backwardMap[rPort] = &backward{
		listener:          listener,
		seqChan:           make(chan uint64),
		backwardStatusMap: make(map[uint64]*backwardStatus),
	}
}

func (m *backwardManager) GetSeqChan(rPort string) (chan uint64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if bw, ok := m.backwardMap[rPort]; ok {
		return bw.seqChan, true
	}
	return nil, false
}

func (m *backwardManager) AddConn(rPort string, seq uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	bw, ok := m.backwardMap[rPort]
	if !ok {
		return false
	}

	m.backwardSeqMap[seq] = rPort
	bw.backwardStatusMap[seq] = &backwardStatus{
		dataChan: make(chan []byte, 5),
	}
	return true
}

func (m *backwardManager) GetDataChan(rPort string, seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bw, ok := m.backwardMap[rPort]
	if !ok {
		return nil, false
	}
	status, ok := bw.backwardStatusMap[seq]
	if !ok {
		return nil, false
	}
	return status.dataChan, true
}

func (m *backwardManager) GetDataChanBySeq(seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rPort, ok := m.backwardSeqMap[seq]
	if !ok {
		return nil, false
	}

	bw, ok := m.backwardMap[rPort]
	if !ok {
		return nil, false
	}
	return bw.backwardStatusMap[seq].dataChan, true
}

func (m *backwardManager) CloseTCP(seq uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rPort, ok := m.backwardSeqMap[seq]
	if !ok {
		return
	}

	close(m.backwardMap[rPort].backwardStatusMap[seq].dataChan)
	delete(m.backwardMap[rPort].backwardStatusMap, seq)
}

func (m *backwardManager) CloseSingle(rPort string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closeSingleLocked(rPort)
}

func (m *backwardManager) CloseSingleAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closeSingleAllLocked()
}

func (m *backwardManager) ForceShutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closeSingleAllLocked()
}

func (m *backwardManager) closeSingleLocked(rPort string) {
	bw := m.backwardMap[rPort]
	bw.listener.Close()
	close(bw.seqChan)

	for seq, status := range bw.backwardStatusMap {
		close(status.dataChan)
		delete(bw.backwardStatusMap, seq)
	}

	delete(m.backwardMap, rPort)

	for seq, p := range m.backwardSeqMap {
		if p == rPort {
			delete(m.backwardSeqMap, seq)
		}
	}
}

func (m *backwardManager) closeSingleAllLocked() {
	for rPort, bw := range m.backwardMap {
		bw.listener.Close()
		close(bw.seqChan)

		for seq, status := range bw.backwardStatusMap {
			close(status.dataChan)
			delete(bw.backwardStatusMap, seq)
		}

		delete(m.backwardMap, rPort)
	}

	for seq := range m.backwardSeqMap {
		delete(m.backwardSeqMap, seq)
	}
}
