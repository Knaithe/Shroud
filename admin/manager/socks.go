package manager

import (
	"net"
	"sync"
)

type socksManager struct {
	mu sync.Mutex

	socksSeq    uint64
	socksSeqMap map[uint64]string // map[seq]uuid  just to speed up searching detail only by seq
	socksMap    map[string]*socks // map[uuid]socks's detail

	SocksMessChan chan interface{}
	SocksReady    chan bool
}

type socks struct {
	addr     string
	port     string
	username string
	password string
	listener net.Listener

	socksStatusMap map[uint64]*socksStatus
}

type socksStatus struct {
	isUDP bool
	tcp   *tcpSocks
	udp   *udpSocks
}

type SocksInfo struct {
	Addr     string
	Port     string
	Username string
	Password string
}

type tcpSocks struct {
	dataChan chan []byte
	conn     net.Conn
}

type udpSocks struct {
	dataChan chan []byte
	listener *net.UDPConn
}

func newSocksManager() *socksManager {
	manager := new(socksManager)
	manager.socksMap = make(map[string]*socks)
	manager.socksSeqMap = make(map[uint64]string)
	manager.SocksMessChan = make(chan interface{}, 5)
	manager.SocksReady = make(chan bool)
	return manager
}

func (m *socksManager) NewSocks(uuid, addr, port, username, password string, listener net.Listener) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.socksMap[uuid]; !ok {
		m.socksMap[uuid] = &socks{
			addr:           addr,
			port:           port,
			username:       username,
			password:       password,
			listener:       listener,
			socksStatusMap: make(map[uint64]*socksStatus),
		}
		return true
	}
	return false
}

func (m *socksManager) AddTCPSocket(uuid string, seq uint64, conn net.Conn) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.socksMap[uuid]; ok {
		m.socksMap[uuid].socksStatusMap[seq] = &socksStatus{
			tcp: &tcpSocks{
				dataChan: make(chan []byte, 5),
				conn:     conn,
			},
		}
		return true
	}
	return false
}

func (m *socksManager) GetSocksSeq(uuid string) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	seq := m.socksSeq
	m.socksSeqMap[seq] = uuid
	m.socksSeq++
	return seq
}

func (m *socksManager) GetTCPDataChan(uuid string, seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.socksMap[uuid]; ok {
		if status, ok := s.socksStatusMap[seq]; ok {
			return status.tcp.dataChan, true
		}
	}
	return nil, false
}

func (m *socksManager) GetUDPDataChan(uuid string, seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.socksMap[uuid]; ok {
		if status, ok := s.socksStatusMap[seq]; ok {
			return status.udp.dataChan, true
		}
	}
	return nil, false
}

func (m *socksManager) GetTCPDataChanBySeq(seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	uuid, ok := m.socksSeqMap[seq]
	if !ok {
		return nil, false
	}

	if status, ok := m.socksMap[uuid].socksStatusMap[seq]; ok {
		return status.tcp.dataChan, true
	}
	return nil, false
}

func (m *socksManager) GetUDPDataChanBySeq(seq uint64) (chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	uuid, ok := m.socksSeqMap[seq]
	if !ok {
		return nil, false
	}

	if status, ok := m.socksMap[uuid].socksStatusMap[seq]; ok {
		return status.udp.dataChan, true
	}
	return nil, false
}

// close TCP include close UDP,cuz UDP's control channel is TCP,if TCP broken,UDP is also forced to be shut down
func (m *socksManager) CloseTCP(seq uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	uuid, ok := m.socksSeqMap[seq]
	if !ok {
		return
	}

	status := m.socksMap[uuid].socksStatusMap[seq]

	// bugfix: In order to avoid data loss,so not close conn&listener here.Thx to @lz520520
	close(status.tcp.dataChan)

	if status.isUDP {
		close(status.udp.dataChan)
	}

	delete(m.socksMap[uuid].socksStatusMap, seq)
}

func (m *socksManager) GetUDPStartInfo(seq uint64) (tcpAddr string, uuid string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	uuid, ok = m.socksSeqMap[seq]
	if !ok {
		return "", "", false
	}

	if status, exists := m.socksMap[uuid].socksStatusMap[seq]; exists {
		return status.tcp.conn.LocalAddr().(*net.TCPAddr).IP.String(), uuid, true
	}
	return "", "", false
}

func (m *socksManager) UpdateUDP(uuid string, seq uint64, listener *net.UDPConn) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.socksMap[uuid]; ok {
		if status, ok := s.socksStatusMap[seq]; ok {
			status.isUDP = true
			status.udp = &udpSocks{
				dataChan: make(chan []byte, 5),
				listener: listener,
			}
			return true
		}
	}
	return false
}

func (m *socksManager) GetSocksInfo(uuid string) (*SocksInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.socksMap[uuid]; ok {
		return &SocksInfo{
			Addr:     s.addr,
			Port:     s.port,
			Username: s.username,
			Password: s.password,
		}, true
	}
	return nil, false
}

func (m *socksManager) CloseSocks(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closeSocksLocked(uuid)
}

func (m *socksManager) ForceShutdown(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.socksMap[uuid]; ok {
		m.closeSocksLocked(uuid)
	}
}

func (m *socksManager) closeSocksLocked(uuid string) {
	m.socksMap[uuid].listener.Close()

	for seq, status := range m.socksMap[uuid].socksStatusMap {
		// bugfix: In order to avoid data loss,so not close conn&listener here.Thx to @lz520520
		close(status.tcp.dataChan)
		if status.isUDP {
			close(status.udp.dataChan)
		}
		delete(m.socksMap[uuid].socksStatusMap, seq)
	}

	for seq, u := range m.socksSeqMap {
		if u == uuid {
			delete(m.socksSeqMap, seq)
		}
	}

	delete(m.socksMap, uuid)
}
