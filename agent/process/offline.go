package process

import (
	"net"

	"Shroud/agent/initial"
	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/share"
)

func upstreamOffline(mgr *manager.Manager, options *initial.Options) {
	if options.Mode == initial.NORMAL_ACTIVE || options.Mode == initial.SOCKS5_PROXY_ACTIVE || options.Mode == initial.HTTP_PROXY_ACTIVE || options.Mode == initial.TOR_PROXY_ACTIVE {
		cleanShutdown()
	}

	forceShutdown(mgr)

	broadcastOfflineMess(mgr)

	var (
		newConn net.Conn
		linkKey []byte
	)
	switch options.Mode {
	case initial.NORMAL_PASSIVE:
		newConn, linkKey = normalPassiveReconn(options)
	case initial.IPTABLES_REUSE_PASSIVE:
		newConn, linkKey = ipTableReusePassiveReconn(options)
	case initial.SO_REUSE_PASSIVE:
		newConn, linkKey = soReusePassiveReconn(options)
	case initial.NORMAL_RECONNECT_ACTIVE:
		newConn, linkKey = normalReconnActiveReconn(options, nil)
	case initial.SOCKS5_PROXY_RECONNECT_ACTIVE:
		proxy := share.NewSocks5Proxy(options.Connect, options.Socks5Proxy, options.Socks5ProxyU, options.Socks5ProxyP)
		newConn, linkKey = normalReconnActiveReconn(options, proxy)
	case initial.HTTP_PROXY_RECONNECT_ACTIVE:
		proxy := share.NewHTTPProxy(options.Connect, options.HttpProxy)
		newConn, linkKey = normalReconnActiveReconn(options, proxy)
	case initial.TOR_PROXY_RECONNECT_ACTIVE:
		proxy := share.NewTorProxy(options.Connect, options.TorProxy)
		newConn, linkKey = normalReconnActiveReconn(options, proxy)
	case initial.TOR_HIDDEN_PASSIVE:
		newConn, linkKey = torHiddenPassiveReconn(options)
	default:
		cleanShutdown()
	}

	global.UpdateGComponent(newConn)
	global.Session.SetLinkKey(linkKey)
	share.ClearPreAuthToken()

	tellAdminReonline(mgr)

	broadcastReonlineMess(mgr)
}

func forceShutdown(mgr *manager.Manager) {
	mgr.BackwardManager.ForceShutdown()
	mgr.ForwardManager.ForceShutdown()
	mgr.SocksManager.ForceShutdown()
}

func broadcastOfflineMess(mgr *manager.Manager) {
	children := mgr.ChildrenManager.GetAllChildren()

	for _, childUUID := range children {
		conn, ok := mgr.ChildrenManager.GetConn(childUUID)
		if !ok {
			continue
		}

		childLinkKey, _ := mgr.ChildrenManager.GetLinkKey(childUUID)

		sMessage := protocol.NewDownMsg(conn, global.G_Component.CryptoKey, childLinkKey, global.G_Component.UUID)

		header := &protocol.Header{
			Sender:      global.G_Component.UUID,
			Accepter:    childUUID,
			MessageType: protocol.UPSTREAMOFFLINE,
			RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
			Route:       protocol.TEMP_ROUTE,
		}

		offlineMess := &protocol.UpstreamOffline{
			OK: 1,
		}

		protocol.ConstructMessage(sMessage, header, offlineMess, false)
		sMessage.SendMessage()
	}
}

func broadcastReonlineMess(mgr *manager.Manager) {
	children := mgr.ChildrenManager.GetAllChildren()

	for _, childUUID := range children {
		conn, ok := mgr.ChildrenManager.GetConn(childUUID)
		if !ok {
			continue
		}

		childLinkKey, _ := mgr.ChildrenManager.GetLinkKey(childUUID)

		sMessage := protocol.NewDownMsg(conn, global.G_Component.CryptoKey, childLinkKey, global.G_Component.UUID)

		header := &protocol.Header{
			Sender:      global.G_Component.UUID,
			Accepter:    childUUID,
			MessageType: protocol.UPSTREAMREONLINE,
			RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
			Route:       protocol.TEMP_ROUTE,
		}

		reOnlineMess := &protocol.UpstreamReonline{
			OK: 1,
		}

		protocol.ConstructMessage(sMessage, header, reOnlineMess, false)
		sMessage.SendMessage()
	}
}

func downStreamOffline(mgr *manager.Manager, options *initial.Options, uuid string) {
	mgr.ChildrenManager.DelChild(uuid)

	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.NODEOFFLINE,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	offlineMess := &protocol.NodeOffline{
		UUIDLen: uint16(len(uuid)),
		UUID:    uuid,
	}

	protocol.ConstructMessage(sMessage, header, offlineMess, false)
	sMessage.SendMessage()
}

func tellAdminReonline(mgr *manager.Manager) {
	children := mgr.ChildrenManager.GetAllChildren()

	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	reheader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.NODEREONLINE,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	for _, childUUID := range children {
		conn, ok := mgr.ChildrenManager.GetConn(childUUID)
		if !ok {
			continue
		}

		reMess := &protocol.NodeReonline{
			ParentUUIDLen: uint16(len(global.G_Component.UUID)),
			ParentUUID:    global.G_Component.UUID,
			UUIDLen:       uint16(len(childUUID)),
			UUID:          childUUID,
			IPLen:         uint16(len(conn.RemoteAddr().String())),
			IP:            conn.RemoteAddr().String(),
		}

		protocol.ConstructMessage(sMessage, reheader, reMess, false)
		sMessage.SendMessage()
	}
}

func DispatchOfflineMess(agent *Agent) {
	for {
		message := <-agent.mgr.OfflineManager.OfflineMessChan

		switch message.(type) {
		case *protocol.UpstreamOffline:
			forceShutdown(agent.mgr)
			broadcastOfflineMess(agent.mgr)
		case *protocol.UpstreamReonline:
			agent.sendMyInfo()
			tellAdminReonline(agent.mgr)
			broadcastReonlineMess(agent.mgr)
		}
	}
}
