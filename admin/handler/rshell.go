package handler

import (
	"fmt"

	"Shroud/admin/manager"
	"Shroud/global"
	"Shroud/protocol"
)

func LetRShell(mgr *manager.Manager, route string, uuid string, port string) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.RSHELLLISTEN,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	listenMess := &protocol.RShellListen{
		PortLen: uint16(len(port)),
		Port:    port,
	}

	protocol.ConstructMessage(sMessage, header, listenMess, false)
	sMessage.SendMessage()
}

func SendRShellData(route string, uuid string, seq uint64, data string) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.RSHELLDATA,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	dataMess := &protocol.RShellData{
		Seq:     seq,
		DataLen: uint64(len(data)),
		Data:    []byte(data),
	}

	protocol.ConstructMessage(sMessage, header, dataMess, false)
	sMessage.SendMessage()
}

func SendRShellFin(route string, uuid string, seq uint64) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.RSHELLFIN,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	finMess := &protocol.RShellFin{Seq: seq}
	protocol.ConstructMessage(sMessage, header, finMess, false)
	sMessage.SendMessage()
}

func StopRShell(mgr *manager.Manager, route string, uuid string) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.RSHELLSTOP,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	stopMess := &protocol.RShellStop{All: 1}
	protocol.ConstructMessage(sMessage, header, stopMess, false)
	sMessage.SendMessage()
}

func DispatchRShellMess(mgr *manager.Manager) {
	for {
		message := <-mgr.RShellManager.RShellMessChan

		switch mess := message.(type) {
		case *protocol.RShellReady:
			mgr.RShellManager.ReadyChan <- (mess.OK == 1)

		case *protocol.RShellConn:
			mgr.RShellManager.ConnChan <- mess.Seq

		case *protocol.RShellData:
			fmt.Print(string(mess.Data))

		case *protocol.RShellFin:
			select {
			case mgr.ConsoleManager.Exit <- true:
			default:
			}

		case *protocol.RShellStopDone:
			mgr.RShellManager.StopDoneChan <- true
			mgr.RShellManager.StopDoneChan <- true
		}
	}
}
