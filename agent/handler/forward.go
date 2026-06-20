package handler

import (
	"net"
	"time"

	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/protocol"
)

type Forward struct {
	Seq  uint64
	Addr string
}

func newForward(seq uint64, addr string) *Forward {
	forward := new(Forward)
	forward.Seq = seq
	forward.Addr = addr
	return forward
}

func (forward *Forward) start(mgr *manager.Manager) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	finHeader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.FORWARDFIN,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	finMess := &protocol.ForwardFin{
		Seq: forward.Seq,
	}

	defer func() {
		protocol.ConstructMessage(sMessage, finHeader, finMess, false)
		sMessage.SendMessage()
	}()

	conn, err := net.DialTimeout("tcp", forward.Addr, 10*time.Second)
	if err != nil {
		return
	}

	if !mgr.ForwardManager.CheckForward(forward.Seq) {
		conn.Close()
		return
	}

	dataChan, ok := mgr.ForwardManager.GetDataChan(forward.Seq)
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
		MessageType: protocol.FORWARDDATA,
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

		forwardDataMess := &protocol.ForwardData{
			Seq:     forward.Seq,
			DataLen: uint64(length),
			Data:    buffer[:length],
		}

		protocol.ConstructMessage(sMessage, dataHeader, forwardDataMess, false)
		sMessage.SendMessage()
	}
}

func testForward(addr string) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.FORWARDREADY,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	succMess := &protocol.ForwardReady{
		OK: 1,
	}

	failMess := &protocol.ForwardReady{
		OK: 0,
	}

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		protocol.ConstructMessage(sMessage, header, failMess, false)
		sMessage.SendMessage()
		return
	}

	conn.Close()

	protocol.ConstructMessage(sMessage, header, succMess, false)
	sMessage.SendMessage()
}

func DispatchForwardMess(mgr *manager.Manager) {
	for {
		message := <-mgr.ForwardManager.ForwardMessChan

		switch mess := message.(type) {
		case *protocol.ForwardTest:
			go testForward(mess.Addr)
		case *protocol.ForwardStart:
			mgr.ForwardManager.NewForward(mess.Seq)
			forward := newForward(mess.Seq, mess.Addr)
			go forward.start(mgr)
		case *protocol.ForwardData:
			dataChan, ok := mgr.ForwardManager.GetDataChan(mess.Seq)
			if ok {
				dataChan <- mess.Data
			}
		case *protocol.ForwardFin:
			mgr.ForwardManager.CloseTCP(mess.Seq)
		}
	}
}
