package handler

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"

	"Shroud/agent/initial"
	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/identity"
	"Shroud/protocol"
	"Shroud/share"
	"Shroud/share/transport"

	reuseport "github.com/libp2p/go-reuseport"
)

const (
	NORMAL = iota
	IPTABLES
	SOREUSE
	TORHIDDEN
)

type Listen struct {
	method int
	addr   string
}

func newListen(method int, addr string) *Listen {
	listen := new(Listen)
	listen.method = method
	listen.addr = addr
	return listen
}

type listenConfig struct {
	createListener   func() (net.Listener, error)
	wrapTLS          bool
	authenticateConn func(conn net.Conn, mgr *manager.Manager, childIP string) ([]byte, identity.Certificate, string, error)
	childIPFunc      func(conn net.Conn) string
}

func tlsAndPreAuth(conn net.Conn, mgr *manager.Manager, childIP string) ([]byte, identity.Certificate, string, error) {
	return passiveChildAuth(conn, mgr, childIP)
}

func soReuseAuth(options *initial.Options) func(net.Conn, *manager.Manager, string) ([]byte, identity.Certificate, string, error) {
	return func(conn net.Conn, mgr *manager.Manager, childIP string) ([]byte, identity.Certificate, string, error) {
		return soReusePassiveChildAuth(conn, options.ReusePort, mgr, childIP)
	}
}

func defaultChildIP(conn net.Conn) string {
	return conn.RemoteAddr().String()
}

func (listen *Listen) normalConfig() listenConfig {
	return listenConfig{
		createListener: func() (net.Listener, error) {
			return net.Listen("tcp", listen.addr)
		},
		wrapTLS:          true,
		authenticateConn: tlsAndPreAuth,
		childIPFunc:      defaultChildIP,
	}
}

func (listen *Listen) iptablesConfig(options *initial.Options) listenConfig {
	return listenConfig{
		createListener: func() (net.Listener, error) {
			initial.SetPortReuseRules(options.Listen, options.ReusePort)
			listenAddr := fmt.Sprintf("0.0.0.0:%s", options.Listen)
			return net.Listen("tcp", listenAddr)
		},
		wrapTLS:          true,
		authenticateConn: tlsAndPreAuth,
		childIPFunc:      defaultChildIP,
	}
}

func (listen *Listen) soReuseConfig(options *initial.Options) listenConfig {
	return listenConfig{
		createListener: func() (net.Listener, error) {
			listenAddr := fmt.Sprintf("%s:%s", options.ReuseHost, options.ReusePort)
			return reuseport.Listen("tcp", listenAddr)
		},
		wrapTLS:          true,
		authenticateConn: soReuseAuth(options),
		childIPFunc:      defaultChildIP,
	}
}

func (listen *Listen) torHiddenConfig(options *initial.Options) (listenConfig, error) {
	if options.TorControl == "" {
		return listenConfig{}, fmt.Errorf("tor control not configured")
	}

	var onionAddr string
	var localPort int

	return listenConfig{
		createListener: func() (net.Listener, error) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				return nil, err
			}
			localPort = listener.Addr().(*net.TCPAddr).Port

			tc := share.NewTorControl(options.TorControl, options.TorControlPW)
			if err := tc.Connect(); err != nil {
				listener.Close()
				return nil, err
			}
			if err := tc.Authenticate(); err != nil {
				listener.Close()
				tc.Close()
				return nil, err
			}
			addr, err := tc.AddOnion(localPort, localPort)
			if err != nil {
				listener.Close()
				tc.Close()
				return nil, err
			}
			onionAddr = addr
			log.Printf("[*] Tor hidden service created: %s:%d\n", onionAddr, localPort)
			return listener, nil
		},
		wrapTLS: false,
		authenticateConn: func(conn net.Conn, mgr *manager.Manager, childIP string) ([]byte, identity.Certificate, string, error) {
			return passiveChildAuth(conn, mgr, childIP)
		},
		childIPFunc: func(_ net.Conn) string {
			return fmt.Sprintf("%s:%d", onionAddr, localPort)
		},
	}, nil
}

func (listen *Listen) start(ctx context.Context, mgr *manager.Manager, options *initial.Options) {
	sUMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	resHeader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.LISTENRES,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	failMess := &protocol.ListenRes{
		OK: 0,
	}

	if listen.method == IPTABLES {
		if options.ReusePort == "" {
			protocol.ConstructMessage(sUMessage, resHeader, failMess, false)
			sUMessage.SendMessage()
			return
		}
	} else if listen.method == SOREUSE {
		if options.ReuseHost == "" {
			protocol.ConstructMessage(sUMessage, resHeader, failMess, false)
			sUMessage.SendMessage()
			return
		}
	}

	var cfg listenConfig
	switch listen.method {
	case NORMAL:
		cfg = listen.normalConfig()
	case IPTABLES:
		cfg = listen.iptablesConfig(options)
	case SOREUSE:
		cfg = listen.soReuseConfig(options)
	case TORHIDDEN:
		var err error
		cfg, err = listen.torHiddenConfig(options)
		if err != nil {
			protocol.ConstructMessage(sUMessage, resHeader, failMess, false)
			sUMessage.SendMessage()
			return
		}
	}

	go listen.runListenLoop(ctx, mgr, cfg)
}

func (listen *Listen) runListenLoop(ctx context.Context, mgr *manager.Manager, cfg listenConfig) {
	sUMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	resHeader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.LISTENRES,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	failMess := &protocol.ListenRes{OK: 0}
	succMess := &protocol.ListenRes{OK: 1}

	listener, err := cfg.createListener()
	if err != nil {
		protocol.ConstructMessage(sUMessage, resHeader, failMess, false)
		sUMessage.SendMessage()
		return
	}

	defer listener.Close()

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	protocol.ConstructMessage(sUMessage, resHeader, succMess, false)
	sUMessage.SendMessage()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			log.Printf("[*] Error occurred: %s\n", err.Error())
			continue
		}

		if cfg.wrapTLS && global.Session.TLSEnable {
			var tlsConfig *tls.Config
			tlsConfig, err = transport.NewServerTLSConfig()
			if err != nil {
				log.Printf("[*] Error occurred: %s", err.Error())
				conn.Close()
				continue
			}
			conn = transport.WrapTLSServerConn(conn, tlsConfig)
		}

		param := new(protocol.NegParam)
		param.Conn = conn
		proto := protocol.NewDownProto(param)
		proto.SNegotiate()

		childIP := cfg.childIPFunc(conn)
		linkKey, peerCert, preassignedUUID, err := cfg.authenticateConn(conn, mgr, childIP)
		if err != nil {
			continue
		}

		rMessage := protocol.NewDownMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.ADMIN_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)
		if err != nil {
			conn.Close()
			continue
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)

			if mmess.Greeting == "Shhh..." && mmess.IsAdmin == 0 {
				var childUUID string

				sLMessage := protocol.NewDownMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.ADMIN_UUID)

				hiMess := &protocol.HIMess{
					GreetingLen: uint16(len("Keep silent")),
					Greeting:    "Keep silent",
					UUIDLen:     uint16(len(protocol.ADMIN_UUID)),
					UUID:        protocol.ADMIN_UUID,
					IsAdmin:     1,
					IsReconnect: 0,
				}

				hiHeader := &protocol.Header{
					Sender:      protocol.ADMIN_UUID,
					Accepter:    protocol.TEMP_UUID,
					MessageType: protocol.HI,
					RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
					Route:       protocol.TEMP_ROUTE,
				}

				protocol.ConstructMessage(sLMessage, hiHeader, hiMess, false)
				sLMessage.SendMessage()

				if mmess.IsReconnect == 0 {
					if preassignedUUID != "" {
						childUUID = preassignedUUID
					} else {
						res, reqErr := requestChildUUID(mgr, childIP, peerCert, false, nil, nil)
						if reqErr != nil {
							conn.Close()
							continue
						}
						childUUID = res.UUID
					}

					uuidHeader := &protocol.Header{
						Sender:      protocol.ADMIN_UUID,
						Accepter:    protocol.TEMP_UUID,
						MessageType: protocol.UUID,
						RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
						Route:       protocol.TEMP_ROUTE,
					}

					uuidMess := &protocol.UUIDMess{
						UUIDLen: uint16(len(childUUID)),
						UUID:    childUUID,
					}

					protocol.ConstructMessage(sLMessage, uuidHeader, uuidMess, false)
					sLMessage.SendMessage()
				} else {
					reheader := &protocol.Header{
						Sender:      global.G_Component.UUID,
						Accepter:    protocol.ADMIN_UUID,
						MessageType: protocol.NODEREONLINE,
						RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
						Route:       protocol.TEMP_ROUTE,
					}

					reMess := &protocol.NodeReonline{
						ParentUUIDLen: uint16(len(global.G_Component.UUID)),
						ParentUUID:    global.G_Component.UUID,
						UUIDLen:       uint16(len(mmess.UUID)),
						UUID:          mmess.UUID,
						IPLen:         uint16(len(conn.RemoteAddr().String())),
						IP:            conn.RemoteAddr().String(),
					}

					protocol.ConstructMessage(sUMessage, reheader, reMess, false)
					sUMessage.SendMessage()

					childUUID = mmess.UUID
				}

				mgr.ChildrenManager.NewChild(childUUID, conn, linkKey)

				mgr.ChildrenManager.ChildComeChan <- &manager.ChildInfo{UUID: childUUID, Conn: conn}

				return
			}
		}

		conn.Close()
	}
}

func DispatchListenMess(ctx context.Context, mgr *manager.Manager, options *initial.Options) {
	for {
		message := <-mgr.ListenManager.ListenMessChan

		switch mess := message.(type) {
		case *protocol.ListenReq:
			listen := newListen(int(mess.Method), mess.Addr)
			go listen.start(ctx, mgr, options)
		case *protocol.ChildUUIDRes:
			mgr.ListenManager.ChildUUIDChan <- mess
		}
	}
}
