package manager

import (
	"sync"

	"Shroud/protocol"
	"Shroud/share"
)

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

type listenManager struct {
	ListenMessChan chan interface{}
	ChildUUIDChan  chan *protocol.ChildUUIDRes
}

func newListenManager() *listenManager {
	manager := new(listenManager)
	manager.ListenMessChan = make(chan interface{}, 5)
	manager.ChildUUIDChan = make(chan *protocol.ChildUUIDRes)
	return manager
}

type connectManager struct {
	ConnectMessChan chan interface{}
}

func newConnectManager() *connectManager {
	manager := new(connectManager)
	manager.ConnectMessChan = make(chan interface{}, 5)
	return manager
}

type offlineManager struct {
	OfflineMessChan chan interface{}
}

func newOfflineManager() *offlineManager {
	manager := new(offlineManager)
	manager.OfflineMessChan = make(chan interface{}, 5)
	return manager
}
