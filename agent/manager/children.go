package manager

import (
	"net"
	"sync"
)

type childrenManager struct {
	mu sync.Mutex

	children      map[string]*child
	ChildComeChan chan *ChildInfo
}

type ChildInfo struct {
	UUID string
	Conn net.Conn
}

type child struct {
	conn    net.Conn
	linkKey []byte
}

func newChildrenManager() *childrenManager {
	manager := new(childrenManager)
	manager.children = make(map[string]*child)
	manager.ChildComeChan = make(chan *ChildInfo)
	return manager
}

func (m *childrenManager) NewChild(uuid string, conn net.Conn, linkKey []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.children[uuid] = &child{conn: conn, linkKey: linkKey}
}

func (m *childrenManager) GetConn(uuid string) (net.Conn, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.children[uuid]; ok {
		return c.conn, true
	}
	return nil, false
}

func (m *childrenManager) GetLinkKey(uuid string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.children[uuid]; ok {
		return c.linkKey, true
	}
	return nil, false
}

func (m *childrenManager) GetAllChildren() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var children []string
	for uuid := range m.children {
		children = append(children, uuid)
	}
	return children
}

func (m *childrenManager) DelChild(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.children, uuid)
}
