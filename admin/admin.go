package main

import (
	"log"
	"net"
	"os"
	"runtime"

	"Shroud/admin/cli"
	"Shroud/admin/initial"
	"Shroud/admin/printer"
	"Shroud/admin/process"
	"Shroud/admin/topology"
	"Shroud/global"
	"Shroud/identity"
	"Shroud/protocol"
	"Shroud/share"
	"Shroud/utils"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func isScriptMode() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--script" || arg == "-script" {
			return true
		}
	}
	return false
}

func main() {
	utils.DisableCoreDump()
	printer.InitPrinter()

	var term cli.Terminal
	if isScriptMode() {
		term = cli.NewScriptTerminal()
	} else {
		term = cli.NewTerminal()
	}
	if err := term.Init(); err != nil {
		log.Fatalf("Failed to init terminal: %v", err)
	}

	done := make(chan struct{})
	go listenCtrlC(term, done)

	initial.ExitCleanup = term.Close

	options := initial.ParseOptions()

	cli.Banner()

	share.GeneratePreAuthToken(options.Secret)
	if len(options.Secret) > 0 {
		identity.SetStorageSecret(options.Secret)
	}
	passphrase := options.Passphrase
	if passphrase == "" {
		passphrase = os.Getenv("SHROUD_PASSPHRASE")
		os.Unsetenv("SHROUD_PASSPHRASE")
	}
	if passphrase != "" {
		identity.SetStorePassphrase([]byte(passphrase))
	}
	adminIDPath := ""
	if options.IdentityDir != "" {
		adminIDPath = options.IdentityDir + string(os.PathSeparator) + "admin_identity.json"
	}
	adminID, err := identity.LoadOrCreateAdmin(adminIDPath)
	if err != nil {
		log.Fatalf("Failed to initialize admin identity: %v", err)
	}
	if options.CAFile != "" {
		ca, caErr := identity.LoadCA(options.CAFile)
		if caErr != nil {
			log.Fatalf("Failed to load CA file: %v", caErr)
		}
		adminID.SetExternalCA(ca)
	}
	if adminID.AdminUUID == "" {
		if len(options.Secret) > 0 {
			adminID.AdminUUID = utils.DeriveUUID(options.Secret, "admin-uuid")
		} else {
			adminID.AdminUUID = utils.GenerateUUID()
		}
		_ = adminID.Save()
	}
	protocol.SetAdminUUID(adminID.AdminUUID)
	if len(share.AuthKey) > 0 {
		protocol.SetTempUUID(utils.DeriveUUID(share.AuthKey, "temp-uuid"))
	}
	if options.Magic == "" {
		if len(adminID.ProtocolMagic) == 4 {
			_ = share.SetMagic(adminID.ProtocolMagic)
		} else if len(share.AuthKey) != 0 {
			magic, _ := share.FingerprintFromAuthKey(share.AuthKey)
			_ = share.SetMagic(magic)
			_ = adminID.SetProtocolFingerprint(magic, "")
		} else {
			log.Fatal("Missing enrollment secret or stored protocol magic; provide -s/SHROUD_SECRET for first run")
		}
	} else {
		_ = adminID.SetProtocolFingerprint(share.Magic(), "")
	}
	if options.WSPath == "" {
		if adminID.WebSocketPath != "" {
			_ = protocol.SetWebSocketPath(adminID.WebSocketPath)
		} else if len(share.AuthKey) != 0 {
			_, wsPath := share.FingerprintFromAuthKey(share.AuthKey)
			_ = protocol.SetWebSocketPath(wsPath)
			_ = adminID.SetProtocolFingerprint(nil, wsPath)
		} else {
			log.Fatal("Missing enrollment secret or stored WebSocket path; provide -s/SHROUD_SECRET for first run")
		}
	} else {
		_ = adminID.SetProtocolFingerprint(nil, protocol.WebSocketPath())
	}
	cryptoKey := []byte(nil)

	if options.PadSize > 0 {
		protocol.SetPadSize(options.PadSize)
	}

	protocol.SetUpDownStream("raw", options.Downstream)

	topo := topology.NewTopology()
	go topo.Run()

	printer.Warning("[*] Waiting for new connection...\r\n")
	var (
		conn    net.Conn
		linkKey []byte
	)
	switch options.Mode {
	case initial.NORMAL_ACTIVE:
		conn, linkKey = initial.NormalActive(options, cryptoKey, topo, nil, adminID)
	case initial.NORMAL_PASSIVE:
		conn, linkKey = initial.NormalPassive(options, cryptoKey, topo, adminID)
	case initial.SOCKS5_PROXY_ACTIVE:
		proxy := share.NewSocks5Proxy(options.Connect, options.Socks5Proxy, options.Socks5ProxyU, options.Socks5ProxyP)
		conn, linkKey = initial.NormalActive(options, cryptoKey, topo, proxy, adminID)
	case initial.HTTP_PROXY_ACTIVE:
		proxy := share.NewHTTPProxy(options.Connect, options.HttpProxy)
		conn, linkKey = initial.NormalActive(options, cryptoKey, topo, proxy, adminID)
	case initial.TOR_PROXY_ACTIVE:
		proxy := share.NewTorProxy(options.Connect, options.TorProxy)
		conn, linkKey = initial.NormalActive(options, cryptoKey, topo, proxy, adminID)
	default:
		printer.Fail("[*] Unknown Mode")
		os.Exit(0)
	}

	for i := range options.Secret {
		options.Secret[i] = 0
	}
	share.ClearPreAuthToken()

	close(done)
	term.Interrupt()

	admin := process.NewAdmin(options, topo)

	topoTask := &topology.TopoTask{
		Mode: topology.CALCULATE,
	}
	topo.TaskChan <- topoTask
	<-topo.ResultChan

	global.InitialGComponent(conn, cryptoKey, protocol.ADMIN_UUID)
	global.Session.LinkKey = linkKey
	global.Session.TLSEnable = options.TlsEnable
	global.Session.TLSFingerprint = options.TlsFingerprint
	global.Session.TLSInsecure = options.TlsInsecure
	global.SetAdminIdentity(adminID)
	protocol.SetSecurityContext(adminID.PayloadKeyForPeerUUID, adminID, nil)

	admin.Run(term)
}

func listenCtrlC(term cli.Terminal, done <-chan struct{}) {
	for {
		ev := term.PollEvent()

		select {
		case <-done:
			return
		default:
		}

		if ev.Key == cli.KeyCtrlC {
			if global.Session != nil && global.Session.AdminIdentity != nil {
				global.Session.AdminIdentity.WipeSeeds()
			}
			term.Close()
			os.Exit(0)
		}
	}
}
