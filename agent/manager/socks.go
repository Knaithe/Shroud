package manager

import (
	"sync"
)

type socksManager struct {
	mu sync.Mutex

	socksStatusMap map[uint64]*socksStatus
	SocksMessChan  chan interface{}
}

type socksStatus struct {
	isUDP bool
	tcp   *tcpSocks
	udp   *udpSocks
}

type tcpSocks struct {
	dataChan chan []byte
}

type udpSocks struct {
	dataChan    chan []byte
	readyChan   chan string
	headerPairs map[string][]byte
}

func newSocksManager() *socksManager {
	manager := new(socksManager)
	manager.socksStatusMap = make(map[uint64]*socksStatus)
	manager.SocksMessChan = make(chan interface{}, 5)
	return manager
}

func (m *socksManager) GetTCPDataChan(seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, ok := m.socksStatusMap[seq]; ok {
		return status.tcp.dataChan, true
	}

	// not found — register it
	m.socksStatusMap[seq] = &socksStatus{
		tcp: &tcpSocks{
			dataChan: make(chan []byte, 5),
		},
	}
	return m.socksStatusMap[seq].tcp.dataChan, false
}

func (m *socksManager) GetUDPChans(seq uint64) (chan []byte, chan string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, ok := m.socksStatusMap[seq]; ok {
		return status.udp.dataChan, status.udp.readyChan, true
	}
	return nil, nil, false
}

func (m *socksManager) GetUDPHeader(seq uint64, addr string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.socksStatusMap[seq]
	if !ok {
		return nil, false
	}
	header, ok := status.udp.headerPairs[addr]
	if !ok {
		return nil, false
	}
	return header, true
}

func (m *socksManager) CheckTCP(seq uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.socksStatusMap[seq]
	return ok
}

func (m *socksManager) CheckUDP(seq uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.socksStatusMap[seq]
	if !ok {
		return false
	}

	status.isUDP = true
	status.udp = &udpSocks{
		dataChan:    make(chan []byte, 5),
		readyChan:   make(chan string),
		headerPairs: make(map[string][]byte),
	}
	return true
}

func (m *socksManager) UpdateUDPHeader(seq uint64, addr string, header []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, ok := m.socksStatusMap[seq]; ok {
		status.udp.headerPairs[addr] = header
	}
}

func (m *socksManager) CloseTCP(seq uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, ok := m.socksStatusMap[seq]
	if !ok {
		return
	}

	close(status.tcp.dataChan)

	if status.isUDP {
		close(status.udp.dataChan)
		close(status.udp.readyChan)
		status.udp.headerPairs = nil
	}

	delete(m.socksStatusMap, seq)
}

func (m *socksManager) CheckSocksReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.socksStatusMap) == 0
}

func (m *socksManager) ForceShutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for seq, status := range m.socksStatusMap {
		close(status.tcp.dataChan)

		if status.isUDP {
			close(status.udp.dataChan)
			close(status.udp.readyChan)
			status.udp.headerPairs = nil
		}

		delete(m.socksStatusMap, seq)
	}
}
