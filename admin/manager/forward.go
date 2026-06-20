package manager

import (
	"net"
	"sync"
)

type forwardManager struct {
	mu sync.Mutex

	forwardSeq      uint64
	forwardSeqMap   map[uint64]*fwSeqRelationship
	forwardMap      map[string]map[string]*forward
	forwardReadyDel map[int]string

	ForwardMessChan chan interface{}
	ForwardReady    chan bool
}

type forward struct {
	remoteAddr string
	listener   net.Listener

	forwardStatusMap map[uint64]*forwardStatus
}

type forwardStatus struct {
	dataChan chan []byte
}

type fwSeqRelationship struct {
	uuid string
	port string
}

type ForwardInfo struct {
	Seq       int
	Laddr     string
	Raddr     string
	ActiveNum int
}

func newForwardManager() *forwardManager {
	manager := new(forwardManager)
	manager.forwardMap = make(map[string]map[string]*forward)
	manager.forwardSeqMap = make(map[uint64]*fwSeqRelationship)
	manager.ForwardMessChan = make(chan interface{}, 5)
	manager.ForwardReady = make(chan bool)
	return manager
}

func (m *forwardManager) NewForward(uuid, port, remoteAddr string, listener net.Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.forwardMap[uuid]; !ok {
		m.forwardMap[uuid] = make(map[string]*forward)
	}
	m.forwardMap[uuid][port] = &forward{
		remoteAddr:       remoteAddr,
		listener:         listener,
		forwardStatusMap: make(map[uint64]*forwardStatus),
	}
}

func (m *forwardManager) GetNewSeq(uuid, port string) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	seq := m.forwardSeq
	m.forwardSeqMap[seq] = &fwSeqRelationship{uuid: uuid, port: port}
	m.forwardSeq++
	return seq
}

func (m *forwardManager) AddConn(uuid, port string, seq uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.forwardSeqMap[seq]; !ok {
		return false
	}
	m.forwardMap[uuid][port].forwardStatusMap[seq] = &forwardStatus{
		dataChan: make(chan []byte, 5),
	}
	return true
}

func (m *forwardManager) GetDataChan(uuid, port string, seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.forwardSeqMap[seq]; !ok {
		return nil, false
	}
	if fs, ok := m.forwardMap[uuid][port].forwardStatusMap[seq]; ok {
		return fs.dataChan, true
	}
	return nil, false
}

func (m *forwardManager) GetDataChanBySeq(seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rel, ok := m.forwardSeqMap[seq]
	if !ok {
		return nil, false
	}
	return m.forwardMap[rel.uuid][rel.port].forwardStatusMap[seq].dataChan, true
}

func (m *forwardManager) GetForwardInfo(uuid string) ([]*ForwardInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.forwardReadyDel = make(map[int]string)

	ports, ok := m.forwardMap[uuid]
	if !ok {
		return nil, false
	}

	var result []*ForwardInfo
	seq := 1
	for port, info := range ports {
		m.forwardReadyDel[seq] = port
		result = append(result, &ForwardInfo{
			Seq:       seq,
			Laddr:     info.listener.Addr().String(),
			Raddr:     info.remoteAddr,
			ActiveNum: len(info.forwardStatusMap),
		})
		seq++
	}
	return result, true
}

func (m *forwardManager) CloseTCP(seq uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rel, ok := m.forwardSeqMap[seq]
	if !ok {
		return
	}
	close(m.forwardMap[rel.uuid][rel.port].forwardStatusMap[seq].dataChan)
	delete(m.forwardMap[rel.uuid][rel.port].forwardStatusMap, seq)
	delete(m.forwardSeqMap, seq)
}

func (m *forwardManager) CloseSingle(uuid string, choice int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeSingleLocked(uuid, m.forwardReadyDel[choice])
}

func (m *forwardManager) CloseSingleAll(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeSingleAllLocked(uuid)
}

func (m *forwardManager) ForceShutdown(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.forwardMap[uuid]; ok {
		m.closeSingleAllLocked(uuid)
	}
}

func (m *forwardManager) closeSingleLocked(uuid, port string) {
	m.forwardMap[uuid][port].listener.Close()

	for seq, status := range m.forwardMap[uuid][port].forwardStatusMap {
		close(status.dataChan)
		delete(m.forwardMap[uuid][port].forwardStatusMap, seq)
	}
	delete(m.forwardMap[uuid], port)

	for seq, rel := range m.forwardSeqMap {
		if rel.uuid == uuid && rel.port == port {
			delete(m.forwardSeqMap, seq)
		}
	}

	if len(m.forwardMap[uuid]) == 0 {
		delete(m.forwardMap, uuid)
	}
}

func (m *forwardManager) closeSingleAllLocked(uuid string) {
	for port, fwd := range m.forwardMap[uuid] {
		fwd.listener.Close()
		for seq, status := range fwd.forwardStatusMap {
			close(status.dataChan)
			delete(fwd.forwardStatusMap, seq)
		}
		delete(m.forwardMap[uuid], port)
	}

	for seq, rel := range m.forwardSeqMap {
		if rel.uuid == uuid {
			delete(m.forwardSeqMap, seq)
		}
	}

	delete(m.forwardMap, uuid)
}
