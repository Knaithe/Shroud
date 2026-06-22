package handler

import (
	"Shroud/admin/manager"
	"Shroud/admin/printer"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/utils"
)

func LetConnect(mgr *manager.Manager, route, uuid, addr string) error {
	normalAddr, _, err := utils.CheckIPPort(addr)
	if err != nil {
		return err
	}

	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.CONNECTSTART,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	connMess := &protocol.ConnectStart{
		AddrLen: uint16(len([]byte(normalAddr))),
		Addr:    normalAddr,
	}

	protocol.ConstructMessage(sMessage, header, connMess, false)
	sMessage.SendMessage()

	if ok := <-mgr.ConnectManager.ConnectReady; !ok {
		printer.Fail("\r\n[*] Cannot connect to node %s", addr)
	}

	return nil
}

func DispatchConnectMess(mgr *manager.Manager) {
	for {
		message := <-mgr.ConnectManager.ConnectMessChan

		switch mess := message.(type) {
		case *protocol.ConnectDone:
			if mess.OK == 1 {
				mgr.ConnectManager.ConnectReady <- true
			} else {
				mgr.ConnectManager.ConnectReady <- false
			}
		}
	}
}
