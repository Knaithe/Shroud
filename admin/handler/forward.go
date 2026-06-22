package handler

import (
	"context"
	"fmt"
	"net"

	"Shroud/admin/manager"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/utils"
)

type Forward struct {
	Addr string
	Port string
}

func NewForward(port, addr string) *Forward {
	forward := new(Forward)
	forward.Port = port
	forward.Addr = addr
	return forward
}

func (forward *Forward) LetForward(ctx context.Context, mgr *manager.Manager, route string, uuid string) error {
	listenAddr := fmt.Sprintf("127.0.0.1:%s", forward.Port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.FORWARDTEST,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	testMess := &protocol.ForwardTest{
		AddrLen: uint16(len([]byte(forward.Addr))),
		Addr:    forward.Addr,
	}

	protocol.ConstructMessage(sMessage, header, testMess, false)
	sMessage.SendMessage()

	if ready := <-mgr.ForwardManager.ForwardReady; !ready {
		listener.Close()
		err := fmt.Errorf("fail to forward port %s to remote addr %s,remote addr is not responding", forward.Port, forward.Addr)
		return err
	}

	mgr.ForwardManager.NewForward(uuid, forward.Port, forward.Addr, listener)

	go forward.handleForwardListener(ctx, mgr, listener, route, uuid)

	return nil
}

func (forward *Forward) handleForwardListener(ctx context.Context, mgr *manager.Manager, listener net.Listener, route string, uuid string) {
	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			listener.Close()
			return
		}

		seq := mgr.ForwardManager.GetNewSeq(uuid, forward.Port)

		if !mgr.ForwardManager.AddConn(uuid, forward.Port, seq) {
			conn.Close()
			return
		}

		go forward.handleForward(mgr, conn, route, uuid, seq)
	}
}

func (forward *Forward) handleForward(mgr *manager.Manager, conn net.Conn, route string, uuid string, seq uint64) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)
	startHeader := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.FORWARDSTART,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	startMess := &protocol.ForwardStart{
		Seq:     seq,
		AddrLen: uint16(len([]byte(forward.Addr))),
		Addr:    forward.Addr,
	}

	dataChan, ok := mgr.ForwardManager.GetDataChan(uuid, forward.Port, seq)

	protocol.ConstructMessage(sMessage, startHeader, startMess, false)
	sMessage.SendMessage()

	defer func() {
		finHeader := &protocol.Header{
			Sender:      protocol.ADMIN_UUID,
			Accepter:    uuid,
			MessageType: protocol.FORWARDFIN,
			RouteLen:    uint32(len([]byte(route))),
			Route:       route,
		}

		finMess := &protocol.ForwardFin{
			Seq: seq,
		}

		protocol.ConstructMessage(sMessage, finHeader, finMess, false)
		sMessage.SendMessage()
	}()

	if !ok {
		return
	}

	go func() {
		for {
			if data, ok := <-dataChan; ok {
				conn.Write(data)
			} else {
				conn.Close()
				return
			}
		}
	}()

	dataHeader := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.FORWARDDATA,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	buffer := make([]byte, 20480)

	for {
		length, err := conn.Read(buffer)
		if err != nil {
			conn.Close()
			return
		}

		forwardDataMess := &protocol.ForwardData{
			Seq:     seq,
			DataLen: uint64(length),
			Data:    buffer[:length],
		}

		protocol.ConstructMessage(sMessage, dataHeader, forwardDataMess, false)
		sMessage.SendMessage()
	}
}

func GetForwardInfo(mgr *manager.Manager, uuid string) (int, bool) {
	info, ok := mgr.ForwardManager.GetForwardInfo(uuid)

	if ok {
		fmt.Print("\r\n[0] All")
		for _, i := range info {
			fmt.Printf(
				"\r\n[%d] Listening Addr: %s , Remote Addr: %s , Active Connections: %d",
				i.Seq,
				i.Laddr,
				i.Raddr,
				i.ActiveNum,
			)
		}
	}

	return len(info) - 1, ok
}

func StopForward(mgr *manager.Manager, uuid string, choice int) {
	if choice == 0 {
		mgr.ForwardManager.CloseSingleAll(uuid)
	} else {
		mgr.ForwardManager.CloseSingle(uuid, choice)
	}
}

func DispatchForwardMess(mgr *manager.Manager) {
	for {
		message := <-mgr.ForwardManager.ForwardMessChan

		switch mess := message.(type) {
		case *protocol.ForwardReady:
			if mess.OK == 1 {
				mgr.ForwardManager.ForwardReady <- true
			} else {
				mgr.ForwardManager.ForwardReady <- false
			}
		case *protocol.ForwardData:
			ch, ok := mgr.ForwardManager.GetDataChanBySeq(mess.Seq)
			if ok {
				utils.SafeSend(ch, mess.Data)
			}
		case *protocol.ForwardFin:
			mgr.ForwardManager.CloseTCP(mess.Seq)
		}
	}
}
