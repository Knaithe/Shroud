package initial

import (
	"crypto/tls"
	"net"
	"os"

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

func NormalActive(userOptions *Options, cryptoKey []byte, topo *topology.Topology, proxy share.Proxy, adminID *identity.AdminStore) (net.Conn, []byte) {

	var sMessage, rMessage protocol.Message

	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len("Shhh...")),
		Greeting:    "Shhh...",
		UUIDLen:     uint16(len(protocol.ADMIN_UUID)),
		UUID:        protocol.ADMIN_UUID,
		IsAdmin:     1,
		IsReconnect: 0,
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
			printer.Fail("[*] Error occurred: %s", err.Error())
			os.Exit(0)
		}

		linkKey, peerCert, err := share.ActiveAdminAuthAndExchange(conn, adminID)
		if err != nil && len(share.AuthKey) != 0 {
			conn.Close()
			conn, err = dialAndNegotiate(userOptions, proxy)
			if err == nil {
				linkKey, peerCert, err = share.ActiveAdminIssueEnrollAndExchange(conn, adminID)
			}
		}
		if err != nil {
			printer.Fail("[*] Error occurred: %s", err.Error())
			os.Exit(0)
		}

		sMessage = protocol.NewDownMsg(conn, cryptoKey, linkKey, protocol.ADMIN_UUID)

		protocol.ConstructMessage(sMessage, header, hiMess, false)
		sMessage.SendMessage()

		rMessage = protocol.NewDownMsg(conn, cryptoKey, linkKey, protocol.ADMIN_UUID)
		fHeader, fMessage, err := protocol.DestructMessage(rMessage)

		if err != nil {
			conn.Close()
			printer.Fail("[*] Fail to connect node %s, Error: %s", conn.RemoteAddr().String(), err.Error())
			os.Exit(0)
		}

		if fHeader.MessageType == protocol.HI {
			mmess := fMessage.(*protocol.HIMess)
			if mmess.Greeting == "Keep silent" && mmess.IsAdmin == 0 {
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
					return conn, linkKey
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
					return conn, linkKey
				}
			}
		}

		conn.Close()
		printer.Fail("[*] Target node looks invalid!\n")
	}
}

func NormalPassive(userOptions *Options, cryptoKey []byte, topo *topology.Topology, adminID *identity.AdminStore) (net.Conn, []byte) {
	listenAddr, _, err := utils.CheckIPPort(userOptions.Listen)
	if err != nil {
		printer.Fail("[*] Error occurred: %s", err.Error())
		os.Exit(0)
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		printer.Fail("[*] Error occurred: %s", err.Error())
		os.Exit(0)
	}

	defer func() {
		listener.Close() // don't forget close the listener
	}()

	var sMessage, rMessage protocol.Message

	// just say hi!
	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len("Keep silent")),
		Greeting:    "Keep silent",
		UUIDLen:     uint16(len(protocol.ADMIN_UUID)),
		UUID:        protocol.ADMIN_UUID,
		IsAdmin:     1,
		IsReconnect: 0,
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
			if mmess.Greeting == "Shhh..." && mmess.IsAdmin == 0 {
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

				return conn, linkKey
			}
		}

		conn.Close()
		printer.Fail("[*] Incoming connection looks invalid.")
	}
}
