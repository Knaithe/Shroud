package manager

import (
	"net"
	"sync"
)

type rshellManager struct {
	mu sync.Mutex

	rshellSeq    uint64
	listener     net.Listener
	connMap      map[uint64]*rshellConn
	RShellMessChan chan interface{}
}

type rshellConn struct {
	conn    net.Conn
	dataChan chan []byte
}

func newRShellManager() *rshellManager {
	m := new(rshellManager)
	m.connMap = make(map[uint64]*rshellConn)
	m.RShellMessChan = make(chan interface{}, 5)
	return m
}

func (m *rshellManager) SetListener(l net.Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listener != nil {
		m.listener.Close()
	}
	m.listener = l
}

func (m *rshellManager) AddConn(conn net.Conn) (uint64, chan []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	seq := m.rshellSeq
	m.rshellSeq++

	dataChan := make(chan []byte, 10)
	m.connMap[seq] = &rshellConn{conn: conn, dataChan: dataChan}
	return seq, dataChan
}

func (m *rshellManager) GetConn(seq uint64) (net.Conn, chan []byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rc, ok := m.connMap[seq]; ok {
		return rc.conn, rc.dataChan, true
	}
	return nil, nil, false
}

func (m *rshellManager) DelConn(seq uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rc, ok := m.connMap[seq]; ok {
		close(rc.dataChan)
		rc.conn.Close()
		delete(m.connMap, seq)
	}
}

func (m *rshellManager) ForceShutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.listener != nil {
		m.listener.Close()
		m.listener = nil
	}

	for seq, rc := range m.connMap {
		close(rc.dataChan)
		rc.conn.Close()
		delete(m.connMap, seq)
	}
}
