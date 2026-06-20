package manager

import (
	"sync"
)

type forwardManager struct {
	mu sync.Mutex

	forwardStatusMap map[uint64]*forwardStatus
	ForwardMessChan  chan interface{}
}

type forwardStatus struct {
	dataChan chan []byte
}

func newForwardManager() *forwardManager {
	manager := new(forwardManager)
	manager.forwardStatusMap = make(map[uint64]*forwardStatus)
	manager.ForwardMessChan = make(chan interface{}, 5)
	return manager
}

func (m *forwardManager) NewForward(seq uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.forwardStatusMap[seq] = &forwardStatus{
		dataChan: make(chan []byte, 5),
	}
}

func (m *forwardManager) GetDataChan(seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, ok := m.forwardStatusMap[seq]; ok {
		return status.dataChan, true
	}
	return nil, false
}

func (m *forwardManager) CheckForward(seq uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.forwardStatusMap[seq]
	return ok
}

func (m *forwardManager) CloseTCP(seq uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, ok := m.forwardStatusMap[seq]; ok {
		close(status.dataChan)
		delete(m.forwardStatusMap, seq)
	}
}

func (m *forwardManager) ForceShutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for seq, status := range m.forwardStatusMap {
		close(status.dataChan)
		delete(m.forwardStatusMap, seq)
	}
}
