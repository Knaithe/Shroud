package manager

import (
	"sync"
)

type backwardManager struct {
	mu sync.Mutex

	backwardSeq      uint64
	backwardSeqMap   map[uint64]*bwSeqRelationship   // map[seq](port+uuid) just to speed up searching detail only by seq
	backwardMap      map[string]map[string]*backward  // map[uuid][rport]backward status
	backwardReadyDel map[int]string

	BackwardMessChan chan interface{}
	BackwardReady    chan bool
}

type backward struct {
	localPort string

	backwardStatusMap map[uint64]*backwardStatus
}

type backwardStatus struct {
	dataChan chan []byte
}

type bwSeqRelationship struct {
	uuid  string
	rPort string
}

type BackwardInfo struct {
	Seq       int
	LPort     string
	RPort     string
	ActiveNum int
}

func newBackwardManager() *backwardManager {
	manager := new(backwardManager)
	manager.backwardMap = make(map[string]map[string]*backward)
	manager.backwardSeqMap = make(map[uint64]*bwSeqRelationship)
	manager.BackwardMessChan = make(chan interface{}, 5)
	manager.BackwardReady = make(chan bool)
	return manager
}

// register a new backward
// 2022.7.19 Fix nil pointer bug,thx to @zyylhn
func (m *backwardManager) NewBackward(uuid, lPort, rPort string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.backwardMap[uuid]; !ok {
		m.backwardMap[uuid] = make(map[string]*backward)
	}

	m.backwardMap[uuid][rPort] = &backward{
		localPort:         lPort,
		backwardStatusMap: make(map[uint64]*backwardStatus),
	}
}

func (m *backwardManager) GetNewSeq(uuid, rPort string) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	seq := m.backwardSeq
	m.backwardSeqMap[seq] = &bwSeqRelationship{uuid: uuid, rPort: rPort}
	m.backwardSeq++
	return seq
}

func (m *backwardManager) AddConn(uuid, rPort string, seq uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.backwardSeqMap[seq]; !ok {
		return false
	}

	m.backwardMap[uuid][rPort].backwardStatusMap[seq] = &backwardStatus{
		dataChan: make(chan []byte, 5),
	}
	return true
}

func (m *backwardManager) CheckBackward(uuid, rPort string, seq uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.backwardSeqMap[seq]; !ok {
		return false
	}

	_, ok := m.backwardMap[uuid][rPort].backwardStatusMap[seq]
	return ok
}

func (m *backwardManager) GetDataChan(uuid, rPort string, seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.backwardSeqMap[seq]; !ok {
		return nil, false
	}

	if bs, ok := m.backwardMap[uuid][rPort].backwardStatusMap[seq]; ok {
		return bs.dataChan, true
	}
	return nil, false
}

func (m *backwardManager) GetDataChanBySeq(seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rel, ok := m.backwardSeqMap[seq]
	if !ok {
		return nil, false
	}

	return m.backwardMap[rel.uuid][rel.rPort].backwardStatusMap[seq].dataChan, true
}

func (m *backwardManager) CloseTCP(seq uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rel, ok := m.backwardSeqMap[seq]
	if !ok {
		return
	}

	close(m.backwardMap[rel.uuid][rel.rPort].backwardStatusMap[seq].dataChan)
	delete(m.backwardMap[rel.uuid][rel.rPort].backwardStatusMap, seq)
	delete(m.backwardSeqMap, seq)
}

func (m *backwardManager) GetBackwardInfo(uuid string) ([]*BackwardInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.backwardReadyDel = make(map[int]string)

	ports, ok := m.backwardMap[uuid]
	if !ok {
		return nil, false
	}

	var result []*BackwardInfo
	seq := 1
	for port, info := range ports {
		m.backwardReadyDel[seq] = port
		result = append(result, &BackwardInfo{
			Seq:       seq,
			LPort:     info.localPort,
			RPort:     port,
			ActiveNum: len(info.backwardStatusMap),
		})
		seq++
	}
	return result, true
}

func (m *backwardManager) GetStopRPort(choice int) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.backwardReadyDel[choice]
}

func (m *backwardManager) CloseSingle(uuid, rPort string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closeSingleLocked(uuid, rPort)
}

func (m *backwardManager) CloseSingleAll(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closeSingleAllLocked(uuid)
}

func (m *backwardManager) ForceShutdown(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.backwardMap[uuid]; ok {
		m.forceShutdownLocked(uuid)
	}
}

func (m *backwardManager) closeSingleLocked(uuid, rPort string) {
	delete(m.backwardMap[uuid], rPort)

	for seq, rel := range m.backwardSeqMap {
		if rel.uuid == uuid && rel.rPort == rPort {
			delete(m.backwardSeqMap, seq)
		}
	}

	if len(m.backwardMap[uuid]) == 0 {
		delete(m.backwardMap, uuid)
	}
}

func (m *backwardManager) closeSingleAllLocked(uuid string) {
	for rPort := range m.backwardMap[uuid] {
		delete(m.backwardMap[uuid], rPort)
	}

	for seq, rel := range m.backwardSeqMap {
		if rel.uuid == uuid {
			delete(m.backwardSeqMap, seq)
		}
	}

	delete(m.backwardMap, uuid)
}

func (m *backwardManager) forceShutdownLocked(uuid string) {
	for rPort := range m.backwardMap[uuid] {
		for seq, status := range m.backwardMap[uuid][rPort].backwardStatusMap {
			close(status.dataChan)
			delete(m.backwardMap[uuid][rPort].backwardStatusMap, seq)
		}
		delete(m.backwardMap[uuid], rPort)
	}

	for seq, rel := range m.backwardSeqMap {
		if rel.uuid == uuid {
			delete(m.backwardSeqMap, seq)
		}
	}

	delete(m.backwardMap, uuid)
}
