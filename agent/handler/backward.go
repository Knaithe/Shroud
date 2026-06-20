package handler

import (
	"context"
	"fmt"
	"net"

	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/protocol"
)

type Backward struct {
	Lport    string
	Rport    string
	Listener net.Listener
}

func newBackward(listener net.Listener, lPort, rPort string) *Backward {
	backward := new(Backward)
	backward.Listener = listener
	backward.Lport = lPort
	backward.Rport = rPort
	return backward
}

func (backward *Backward) start(ctx context.Context, mgr *manager.Manager) {
	mgr.BackwardManager.NewBackward(backward.Rport, backward.Listener)

	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	go func() {
		<-ctx.Done()
		backward.Listener.Close()
	}()

	for {
		conn, err := backward.Listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			backward.Listener.Close()
			return
		}

		seqHeader := &protocol.Header{
			Sender:      global.G_Component.UUID,
			Accepter:    protocol.ADMIN_UUID,
			MessageType: protocol.BACKWARDSTART,
			RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
			Route:       protocol.TEMP_ROUTE,
		}

		seqMess := &protocol.BackwardStart{
			UUIDLen:  uint16(len(global.G_Component.UUID)),
			UUID:     global.G_Component.UUID,
			LPortLen: uint16(len(backward.Lport)),
			LPort:    backward.Lport,
			RPortLen: uint16(len(backward.Rport)),
			RPort:    backward.Rport,
		}

		protocol.ConstructMessage(sMessage, seqHeader, seqMess, false)
		sMessage.SendMessage()

		seqChan, ok := mgr.BackwardManager.GetSeqChan(backward.Rport)
		if !ok {
			conn.Close()
			return
		}

		seq, ok := <-seqChan
		if !ok {
			conn.Close()
			return
		}

		go backward.handleBackward(mgr, conn, seq)
	}
}

func (backward *Backward) handleBackward(mgr *manager.Manager, conn net.Conn, seq uint64) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	defer func() {
		finHeader := &protocol.Header{
			Sender:      global.G_Component.UUID,
			Accepter:    protocol.ADMIN_UUID,
			MessageType: protocol.BACKWARDFIN,
			RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
			Route:       protocol.TEMP_ROUTE,
		}

		finMess := &protocol.BackWardFin{
			Seq: seq,
		}

		protocol.ConstructMessage(sMessage, finHeader, finMess, false)
		sMessage.SendMessage()
	}()

	ok := mgr.BackwardManager.AddConn(backward.Rport, seq)
	mgr.BackwardManager.SeqReady <- true
	if !ok {
		conn.Close()
		return
	}

	// ask for corresponding datachan
	dataChan, ok := mgr.BackwardManager.GetDataChan(backward.Rport, seq)
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
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.BACKWARDDATA,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	buffer := make([]byte, 20480)

	for {
		length, err := conn.Read(buffer)
		if err != nil {
			conn.Close()
			return
		}

		backwardDataMess := &protocol.BackwardData{
			Seq:     seq,
			DataLen: uint64(length),
			Data:    buffer[:length],
		}

		protocol.ConstructMessage(sMessage, dataHeader, backwardDataMess, false)
		sMessage.SendMessage()
	}
}

func testBackward(ctx context.Context, mgr *manager.Manager, lPort, rPort string) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.BACKWARDREADY,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	succMess := &protocol.BackwardReady{
		OK: 1,
	}

	failMess := &protocol.BackwardReady{
		OK: 0,
	}

	listenAddr := fmt.Sprintf("127.0.0.1:%s", rPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		protocol.ConstructMessage(sMessage, header, failMess, false)
		sMessage.SendMessage()
		return
	}

	backward := newBackward(listener, lPort, rPort)

	go backward.start(ctx, mgr)

	protocol.ConstructMessage(sMessage, header, succMess, false)
	sMessage.SendMessage()
}

func sendDoneMess(all uint16, rPort string) {
	// here is a problem,if some of the backward conns cannot send FIN before DONE,then the FIN they send cannot be processed by admin
	// but it's not a really big problem,because users must know some data maybe lost since they choose to close backward
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.BACKWARDSTOPDONE,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	doneMess := &protocol.BackwardStopDone{
		All:      all,
		UUIDLen:  uint16(len(global.G_Component.UUID)),
		UUID:     global.G_Component.UUID,
		RPortLen: uint16(len(rPort)),
		RPort:    rPort,
	}

	protocol.ConstructMessage(sMessage, header, doneMess, false)
	sMessage.SendMessage()
}

func DispatchBackwardMess(ctx context.Context, mgr *manager.Manager) {
	for {
		message := <-mgr.BackwardManager.BackwardMessChan

		switch mess := message.(type) {
		case *protocol.BackwardTest:
			go testBackward(ctx, mgr, mess.LPort, mess.RPort)
		case *protocol.BackwardSeq:
			seqChan, ok := mgr.BackwardManager.GetSeqChan(mess.RPort)

			if ok {
				seqChan <- mess.Seq
				<-mgr.BackwardManager.SeqReady
			} else {
				sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

				finHeader := &protocol.Header{
					Sender:      global.G_Component.UUID,
					Accepter:    protocol.ADMIN_UUID,
					MessageType: protocol.BACKWARDFIN,
					RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
					Route:       protocol.TEMP_ROUTE,
				}

				finMess := &protocol.BackWardFin{
					Seq: mess.Seq,
				}

				protocol.ConstructMessage(sMessage, finHeader, finMess, false)
				sMessage.SendMessage()
			}
		case *protocol.BackwardData:
			dataChan, ok := mgr.BackwardManager.GetDataChanBySeq(mess.Seq)
			if ok {
				dataChan <- mess.Data
			}
		case *protocol.BackWardFin:
			mgr.BackwardManager.CloseTCP(mess.Seq)
		case *protocol.BackwardStop:
			if mess.All == 1 {
				mgr.BackwardManager.CloseSingleAll()
			} else {
				mgr.BackwardManager.CloseSingle(mess.RPort)
			}
			go sendDoneMess(mess.All, mess.RPort)
		}
	}
}
