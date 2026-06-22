package handler

import (
	"Shroud/global"
	"Shroud/protocol"
)

func LetShutdown(route string, uuid string) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.SHUTDOWN,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	shutdownMess := &protocol.Shutdown{
		OK: 1,
	}

	protocol.ConstructMessage(sMessage, header, shutdownMess, false)
	sMessage.SendMessage()
}
