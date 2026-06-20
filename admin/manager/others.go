package manager

import (
	"sync"

	"Shroud/share"
)

type consoleManager struct {
	OK   chan bool
	Exit chan bool
}

func newConsoleManager() *consoleManager {
	manager := new(consoleManager)
	manager.OK = make(chan bool)
	manager.Exit = make(chan bool, 1)
	return manager
}

type fileManager struct {
	mu           sync.Mutex
	transfers    map[uint64]*share.MyFile
	nextID       uint64
	FileMessChan chan interface{}
}

func newFileManager() *fileManager {
	manager := new(fileManager)
	manager.transfers = make(map[uint64]*share.MyFile)
	manager.nextID = 1
	manager.FileMessChan = make(chan interface{}, 5)
	return manager
}

func (fm *fileManager) NewTransfer() *share.MyFile {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	id := fm.nextID
	fm.nextID++
	file := share.NewFile()
	file.TransferID = id
	fm.transfers[id] = file
	return file
}

func (fm *fileManager) GetTransfer(id uint64) (*share.MyFile, bool) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	file, ok := fm.transfers[id]
	return file, ok
}

func (fm *fileManager) StoreTransfer(id uint64, file *share.MyFile) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.transfers[id] = file
}

func (fm *fileManager) RemoveTransfer(id uint64) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	delete(fm.transfers, id)
}

type sshManager struct {
	SSHMessChan chan interface{}
}

func newSSHManager() *sshManager {
	manager := new(sshManager)
	manager.SSHMessChan = make(chan interface{}, 5)
	return manager
}

type sshTunnelManager struct {
	SSHTunnelMessChan chan interface{}
}

func newSSHTunnelManager() *sshTunnelManager {
	manager := new(sshTunnelManager)
	manager.SSHTunnelMessChan = make(chan interface{}, 5)
	return manager
}

type shellManager struct {
	ShellMessChan chan interface{}
}

func newShellManager() *shellManager {
	manager := new(shellManager)
	manager.ShellMessChan = make(chan interface{}, 5)
	return manager
}

type infoManager struct {
	InfoMessChan chan interface{}
}

func newInfoManager() *infoManager {
	manager := new(infoManager)
	manager.InfoMessChan = make(chan interface{}, 5)
	return manager
}

type listenManager struct {
	ListenMessChan chan interface{}
	ListenReady    chan bool
}

func newListenManager() *listenManager {
	manager := new(listenManager)
	manager.ListenMessChan = make(chan interface{}, 5)
	manager.ListenReady = make(chan bool)
	return manager
}

type connectManager struct {
	ConnectMessChan chan interface{}
	ConnectReady    chan bool
}

func newConnectManager() *connectManager {
	manager := new(connectManager)
	manager.ConnectMessChan = make(chan interface{}, 5)
	manager.ConnectReady = make(chan bool)
	return manager
}

type childrenManager struct {
	ChildrenMessChan chan interface{}
}

func newchildrenManager() *childrenManager {
	manager := new(childrenManager)
	manager.ChildrenMessChan = make(chan interface{}, 5)
	return manager
}

type transportManager struct {
	TransportMessChan chan interface{}
}

func newTransportManager() *transportManager {
	manager := new(transportManager)
	manager.TransportMessChan = make(chan interface{}, 5)
	return manager
}
