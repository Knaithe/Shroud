package manager

type Manager struct {
	ChildrenManager  *childrenManager
	FileManager      *fileManager
	SocksManager     *socksManager
	ForwardManager   *forwardManager
	BackwardManager  *backwardManager
	SSHManager       *sshManager
	SSHTunnelManager *sshTunnelManager
	ShellManager     *shellManager
	ListenManager    *listenManager
	ConnectManager   *connectManager
	OfflineManager   *offlineManager
	RShellManager    *rshellManager
}

func NewManager() *Manager {
	manager := new(Manager)
	manager.ChildrenManager = newChildrenManager()
	manager.FileManager = newFileManager()
	manager.SocksManager = newSocksManager()
	manager.ForwardManager = newForwardManager()
	manager.BackwardManager = newBackwardManager()
	manager.SSHManager = newSSHManager()
	manager.SSHTunnelManager = newSSHTunnelManager()
	manager.ShellManager = newShellManager()
	manager.ListenManager = newListenManager()
	manager.ConnectManager = newConnectManager()
	manager.OfflineManager = newOfflineManager()
	manager.RShellManager = newRShellManager()
	return manager
}

func (manager *Manager) Run() {
}
