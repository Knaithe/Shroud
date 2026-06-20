package manager

type Manager struct {
	ConsoleManager   *consoleManager
	FileManager      *fileManager
	SocksManager     *socksManager
	ForwardManager   *forwardManager
	BackwardManager  *backwardManager
	SSHManager       *sshManager
	SSHTunnelManager *sshTunnelManager
	ShellManager     *shellManager
	InfoManager      *infoManager
	ListenManager    *listenManager
	ConnectManager   *connectManager
	ChildrenManager  *childrenManager
	TransportManager *transportManager
}

func NewManager() *Manager {
	manager := new(Manager)
	manager.ConsoleManager = newConsoleManager()
	manager.FileManager = newFileManager()
	manager.SocksManager = newSocksManager()
	manager.ForwardManager = newForwardManager()
	manager.BackwardManager = newBackwardManager()
	manager.SSHManager = newSSHManager()
	manager.SSHTunnelManager = newSSHTunnelManager()
	manager.ShellManager = newShellManager()
	manager.InfoManager = newInfoManager()
	manager.ListenManager = newListenManager()
	manager.ConnectManager = newConnectManager()
	manager.ChildrenManager = newchildrenManager()
	manager.TransportManager = newTransportManager()
	return manager
}

func (manager *Manager) Run() {
}
