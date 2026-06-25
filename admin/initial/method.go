package initial

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"time"

	"Shroud/admin/printer"
	"Shroud/admin/topology"
	"Shroud/identity"
	"Shroud/protocol"
	"Shroud/share"
	"Shroud/share/transport"
	"Shroud/utils"
)

func dialAndNegotiate(userOptions *Options, proxy share.Proxy) (net.Conn, error) {
	var (
		conn net.Conn
		err  error
	)
	if proxy == nil {
		conn, err = net.Dial("tcp", userOptions.Connect)
	} else {
		conn, err = proxy.Dial()
	}
	if err != nil {
		return nil, err
	}
	utils.EnableKeepAlive(conn)
	if userOptions.TlsEnable {
		var tlsConfig *tls.Config
		tlsConfig, err = transport.NewClientTLSConfig(userOptions.Domain, userOptions.TlsFingerprint, userOptions.TlsInsecure)
		if err != nil {
			conn.Close()
			return nil, err
		}
		conn = transport.WrapTLSClientConn(conn, tlsConfig)
	}
	param := new(protocol.NegParam)
	param.Conn = conn
	param.Domain = userOptions.Domain
	proto := protocol.NewDownProto(param)
	if err := proto.CNegotiate(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func dispatchUUID(conn net.Conn, cryptoKey, linkKey []byte) string {
	var sMessage protocol.Message

	uuid := utils.GenerateUUID()
	uuidMess := &protocol.UUIDMess{
		UUIDLen: uint16(len(uuid)),
		UUID:    uuid,
	}

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    protocol.TEMP_UUID,
		MessageType: protocol.UUID,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	sMessage = protocol.NewDownMsg(conn, cryptoKey, linkKey, protocol.ADMIN_UUID)

	protocol.ConstructMessage(sMessage, header, uuidMess, false)
	sMessage.SendMessage()

	return uuid
}

func NormalActive(userOptions *Options, cryptoKey []byte, topo *topology.Topology, proxy share.Proxy, adminID *identity.AdminStore) (net.Conn, []byte, error) {

	var sMessage, rMessage protocol.Message

	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len(share.GreetHello())),
		Greeting:    share.GreetHello(),
		UUIDLen:     uint16(len(protocol.ADMIN_UUID)),
		UUID:        protocol.ADMIN_UUID,
		IsAdmin:     1,
		IsReconnect: 0,
		VersionLen:  uint16(len(protocol.SHROUD_VERSION)),
		Version:     protocol.SHROUD_VERSION,
	}

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    protocol.TEMP_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for {
		conn, err := dialAndNegotiate(userOptions, proxy)

		if err != nil {
			return nil, nil, fmt.Errorf("dial failed: %w", err)
		}

		linkKey, peerCert, err := share.ActiveAdminAuthAndExchange(conn, adminID)
		if err != nil && len(share.AuthKey) != 0 && errors.Is(err, share.ErrPeerNoCert) {
			conn.Close()
			conn, err = dialAndNegotiate(userOptions, proxy)
			if err == nil {
				linkKey, peerCert, err = share.ActiveAdminIssueEnrollAndExchange(conn, adminID)
			}
		}
		if err != nil {
			if conn != nil {
				conn.Close()
			}
			return nil, nil, fmt.Errorf("auth exchange failed: %w", err)
		}

		sMessage = protocol.NewDownMsg(conn, cryptoKey, linkKey, protocol.ADMIN_UUID)

		protocol.ConstructMessage(sMessage, header, hiMess, false)
		sMessage.SendMessage()

		rMessage = protocol.NewDownMsg(conn, cryptoKey, linkKey, protocol.ADMIN_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			conn.Close()
			return nil, nil, fmt.Errorf("handshake with %s failed: %w", conn.RemoteAddr().String(), err)
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == share.GreetAck() && mmess.IsAdmin == 0 {
				logVersionCheck(mmess.Version, conn.RemoteAddr().String())
				if mmess.IsReconnect == 0 {
					assignedUUID := dispatchUUID(conn, cryptoKey, linkKey)
					if len(peerCert.Signature) != 0 {
						_ = adminID.BindProtocolUUID(assignedUUID, peerCert)
					}
					node := topology.NewNode(assignedUUID, conn.RemoteAddr().String())
					task := &topology.TopoTask{
						Mode:       topology.ADDNODE,
						Target:     node,
						ParentUUID: protocol.TEMP_UUID,
						IsFirst:    true,
					}
					topo.TaskChan <- task

					<-topo.ResultChan

					printer.Success("[*] Connect to node %s successfully! Node id is 0\r\n", conn.RemoteAddr().String())
					return conn, linkKey, nil
				} else {
					if len(peerCert.Signature) != 0 {
						_ = adminID.BindProtocolUUID(mmess.UUID, peerCert)
					}
					node := topology.NewNode(mmess.UUID, conn.RemoteAddr().String())
					task := &topology.TopoTask{
						Mode:       topology.ADDNODE,
						Target:     node,
						ParentUUID: protocol.TEMP_UUID,
						IsFirst:    true,
					}
					topo.TaskChan <- task

					<-topo.ResultChan

					printer.Success("[*] Connect to node %s successfully! Node id is 0\r\n", conn.RemoteAddr().String())
					return conn, linkKey, nil
				}
			}
		}

		conn.Close()
		printer.Fail("[*] Target node looks invalid!\n")
	}
}

func NormalPassive(userOptions *Options, cryptoKey []byte, topo *topology.Topology, adminID *identity.AdminStore) (net.Conn, []byte, error) {
	listenAddr, _, err := utils.CheckIPPort(userOptions.Listen)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid listen address: %w", err)
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen failed: %w", err)
	}

	defer func() {
		listener.Close() // don't forget close the listener
	}()

	var sMessage, rMessage protocol.Message

	// just say hi!
	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len(share.GreetAck())),
		Greeting:    share.GreetAck(),
		UUIDLen:     uint16(len(protocol.ADMIN_UUID)),
		UUID:        protocol.ADMIN_UUID,
		IsAdmin:     1,
		IsReconnect: 0,
		VersionLen:  uint16(len(protocol.SHROUD_VERSION)),
		Version:     protocol.SHROUD_VERSION,
	}

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    protocol.TEMP_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			printer.Fail("[*] Error occurred: %s\r\n", err.Error())
			continue
		}
		utils.EnableKeepAlive(conn)

		if userOptions.TlsEnable {
			var tlsConfig *tls.Config
			tlsConfig, err = transport.NewServerTLSConfig()
			if err != nil {
				printer.Fail("[*] Error occurred: %s", err.Error())
				conn.Close()
				continue
			}
			conn = transport.WrapTLSServerConn(conn, tlsConfig)
		}

		param := new(protocol.NegParam)
		param.Conn = conn
		proto := protocol.NewDownProto(param)
		proto.SNegotiate()

		linkKey, peerCert, err := share.PassiveAdminAuthAndExchange(conn, adminID)
		if err == nil {
			_ = adminID.BindProtocolUUID(peerCert.Serial, peerCert)
		}
		if err != nil {
			printer.Fail("[*] Error occurred: %s\r\n", err.Error())
			conn.Close()
			continue
		}

		rMessage = protocol.NewDownMsg(conn, cryptoKey, linkKey, protocol.ADMIN_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			printer.Fail("[*] Fail to set connection from %s, Error: %s\r\n", conn.RemoteAddr().String(), err.Error())
			conn.Close()
			continue
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == share.GreetHello() && mmess.IsAdmin == 0 {
				logVersionCheck(mmess.Version, conn.RemoteAddr().String())
				sMessage = protocol.NewDownMsg(conn, cryptoKey, linkKey, protocol.ADMIN_UUID)
				protocol.ConstructMessage(sMessage, header, hiMess, false)
				sMessage.SendMessage()

				if mmess.IsReconnect == 0 {
					assignedUUID := dispatchUUID(conn, cryptoKey, linkKey)
					if len(peerCert.Signature) != 0 {
						_ = adminID.BindProtocolUUID(assignedUUID, peerCert)
					}
					node := topology.NewNode(assignedUUID, conn.RemoteAddr().String())
					task := &topology.TopoTask{
						Mode:       topology.ADDNODE,
						Target:     node,
						ParentUUID: protocol.TEMP_UUID,
						IsFirst:    true,
					}
					topo.TaskChan <- task

					<-topo.ResultChan

					printer.Success("[*] Connection from node %s is set up successfully! Node id is 0\r\n", conn.RemoteAddr().String())
				} else {
					if len(peerCert.Signature) != 0 {
						_ = adminID.BindProtocolUUID(mmess.UUID, peerCert)
					}
					node := topology.NewNode(mmess.UUID, conn.RemoteAddr().String())
					task := &topology.TopoTask{
						Mode:       topology.ADDNODE,
						Target:     node,
						ParentUUID: protocol.TEMP_UUID,
						IsFirst:    true,
					}
					topo.TaskChan <- task

					<-topo.ResultChan

					printer.Success("[*] Connection from node %s is set up successfully! Node id is 0\r\n", conn.RemoteAddr().String())
				}

				return conn, linkKey, nil
			}
		}

		conn.Close()
		printer.Fail("[*] Incoming connection looks invalid.")
	}
}

type ReconnectContext struct {
	Options   *Options
	CryptoKey []byte
	Proxy     share.Proxy
	AdminID   *identity.AdminStore
	Daemon    bool
}

func ActiveReconnect(ctx *ReconnectContext) (net.Conn, []byte, error) {
	maxAttempts := 10
	if ctx.Daemon {
		maxAttempts = 10000
	}

	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len(share.GreetHello())),
		Greeting:    share.GreetHello(),
		UUIDLen:     uint16(len(protocol.ADMIN_UUID)),
		UUID:        protocol.ADMIN_UUID,
		IsAdmin:     1,
		IsReconnect: 1,
		VersionLen:  uint16(len(protocol.SHROUD_VERSION)),
		Version:     protocol.SHROUD_VERSION,
	}
	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    protocol.TEMP_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			d := reconnBackoff(attempt)
			printer.Warning("[*] Reconnect attempt %d/%d in %v...\r\n", attempt+1, maxAttempts, d.Round(time.Second))
			time.Sleep(d)
		}

		conn, err := dialAndNegotiate(ctx.Options, ctx.Proxy)
		if err != nil {
			printer.Fail("[*] Reconnect dial failed: %s\r\n", err.Error())
			continue
		}

		linkKey, _, err := share.ActiveAdminAuthAndExchange(conn, ctx.AdminID)
		if err != nil {
			conn.Close()
			printer.Fail("[*] Reconnect auth failed: %s\r\n", err.Error())
			continue
		}

		sMessage := protocol.NewDownMsg(conn, ctx.CryptoKey, linkKey, protocol.ADMIN_UUID)
		protocol.ConstructMessage(sMessage, header, hiMess, false)
		sMessage.SendMessage()

		rMessage := protocol.NewDownMsg(conn, ctx.CryptoKey, linkKey, protocol.ADMIN_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)
		if err != nil {
			conn.Close()
			continue
		}

		if fHeader.MessageType == protocol.HI {
			mmess, ok := fMessage.(*protocol.HIMess)
			if !ok {
				conn.Close()
				continue
			}
			if mmess.Greeting == share.GreetAck() && mmess.IsAdmin == 0 {
				printer.Success("[*] Reconnected successfully!\r\n")
				return conn, linkKey, nil
			}
		}
		conn.Close()
	}
	return nil, nil, fmt.Errorf("reconnection failed after %d attempts", maxAttempts)
}

func logVersionCheck(peerVersion, addr string) {
	if peerVersion == "" {
		printer.Warning("[*] Peer %s is running an older version without version negotiation\r\n", addr)
	} else if peerVersion != protocol.SHROUD_VERSION {
		printer.Warning("[*] Version mismatch with %s: local=%s remote=%s\r\n", addr, protocol.SHROUD_VERSION, peerVersion)
	}
}

func reconnBackoff(attempt int) time.Duration {
	base := 2 * time.Second
	d := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	if d > 5*time.Minute {
		d = 5 * time.Minute
	}
	var b [8]byte
	rand.Read(b[:])
	f := float64(binary.BigEndian.Uint64(b[:])) / float64(math.MaxUint64)
	jitter := time.Duration(f * 0.3 * float64(d))
	return d + jitter
}
