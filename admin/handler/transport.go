package handler

import (
	"net"
	"time"

	"Shroud/admin/manager"
	"Shroud/admin/printer"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/share"
	"Shroud/share/transport"
)

const (
	TRANSPORT_RAW = iota
	TRANSPORT_TOR
)

func SwitchTransport(mgr *manager.Manager, route, uuid string, method int, torProxy string) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.TRANSPORTSWITCHREQ,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	switchReq := &protocol.TransportSwitchReq{
		Method:  uint16(method),
		AddrLen: 0,
		Addr:    "",
	}

	protocol.ConstructMessage(sMessage, header, switchReq, false)
	sMessage.SendMessage()
}

func HandleTransportSwitchRes(mgr *manager.Manager, method int, torProxy string) bool {
	var mess *protocol.TransportSwitchRes

	select {
	case message := <-mgr.TransportManager.TransportMessChan:
		var ok bool
		mess, ok = message.(*protocol.TransportSwitchRes)
		if !ok {
			printer.Fail("\r\n[*] Unexpected message type during transport switch!")
			return false
		}
	case <-time.After(30 * time.Second):
		printer.Fail("\r\n[*] Transport switch timed out (30s)!")
		return false
	}

	if mess.OK == 0 {
		printer.Fail("\r\n[*] Agent rejected transport switch!")
		return false
	}

	agentListenAddr := mess.Addr

	var (
		conn net.Conn
		err  error
	)

	if method == TRANSPORT_TOR {
		proxy := share.NewTorProxy(agentListenAddr, torProxy)
		conn, err = proxy.Dial()
	} else {
		conn, err = net.DialTimeout("tcp", agentListenAddr, 10*time.Second)
	}

	if err != nil {
		printer.Fail("\r\n[*] Failed to establish new connection: %s", err.Error())
		return false
	}

	if global.Session.TLSEnable {
		tlsConfig, err := transport.NewClientTLSConfig("", global.Session.TLSFingerprint, global.Session.TLSInsecure)
		if err != nil {
			conn.Close()
			printer.Fail("\r\n[*] TLS error: %s", err.Error())
			return false
		}
		conn = transport.WrapTLSClientConn(conn, tlsConfig)
	}

	linkKey, _, err := share.ActiveAdminAuthAndExchange(conn, global.Session.AdminIdentity)
	if err != nil {
		conn.Close()
		printer.Fail("\r\n[*] Auth failed on new connection: %s", err.Error())
		return false
	}

	sMessage := protocol.NewDownMsg(conn, global.G_Component.CryptoKey, linkKey, global.G_Component.UUID)

	doneHeader := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    protocol.TEMP_UUID,
		MessageType: protocol.TRANSPORTSWITCHDONE,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	doneMess := &protocol.TransportSwitchDone{
		OK: 1,
	}

	protocol.ConstructMessage(sMessage, doneHeader, doneMess, false)
	sMessage.SendMessage()

	if method == TRANSPORT_TOR {
		global.SetTransportMode("tor")
	} else {
		global.SetTransportMode("raw")
	}

	oldConn := global.SwapGComponentConn(conn)
	global.Session.SetLinkKey(linkKey)
	global.SignalTransportSwitch()
	oldConn.Close()

	if method == TRANSPORT_TOR {
		printer.Success("\r\n[*] Transport switched to Tor successfully!")
	} else {
		printer.Success("\r\n[*] Transport switched to raw TCP successfully!")
	}

	return true
}
