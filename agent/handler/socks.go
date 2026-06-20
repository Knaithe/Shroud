package handler

import (
	"net"

	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/protocol"
)

type Socks struct {
	Username string
	Password string
}

type Setting struct {
	method       string
	isAuthed     bool
	tcpConnected bool
	isUDP        bool
	success      bool
	tcpConn      net.Conn
	udpListener  *net.UDPConn
}

func newSocks() *Socks {
	return new(Socks)
}

func (socks *Socks) start(mgr *manager.Manager) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.SOCKSREADY,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))), // No need to set route when agent send mess to admin
		Route:       protocol.TEMP_ROUTE,
	}

	succMess := &protocol.SocksReady{
		OK: 1,
	}

	failMess := &protocol.SocksReady{
		OK: 0,
	}

	if !mgr.SocksManager.CheckSocksReady() {
		protocol.ConstructMessage(sMessage, header, failMess, false)
		sMessage.SendMessage()
		return
	}

	protocol.ConstructMessage(sMessage, header, succMess, false)
	sMessage.SendMessage()
}

func (socks *Socks) handleSocks(mgr *manager.Manager, dataChan chan []byte, seq uint64) {
	setting := new(Setting)

	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	defer func() { // no matter what happened, after the function return,tell admin that works done
		finHeader := &protocol.Header{
			Sender:      global.G_Component.UUID,
			Accepter:    protocol.ADMIN_UUID,
			MessageType: protocol.SOCKSTCPFIN,
			RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))), // No need to set route when agent send mess to admin
			Route:       protocol.TEMP_ROUTE,
		}

		finMess := &protocol.SocksTCPFin{
			Seq: seq,
		}

		protocol.ConstructMessage(sMessage, finHeader, finMess, false)
		sMessage.SendMessage()
	}()

	for {
		if !setting.isAuthed && setting.method == "" {
			data, ok := <-dataChan
			if !ok {
				return
			}
			socks.checkMethod(setting, data, seq)
		} else if !setting.isAuthed && setting.method == "PASSWORD" {
			data, ok := <-dataChan
			if !ok {
				return
			}

			socks.auth(setting, data, seq)
		} else if setting.isAuthed && !setting.tcpConnected && !setting.isUDP {
			data, ok := <-dataChan
			if !ok {
				return
			}

			socks.buildConn(mgr, setting, data, seq)

			if !setting.tcpConnected && !setting.isUDP {
				return
			}
		} else if setting.isAuthed && setting.tcpConnected && !setting.isUDP { //All done!
			go proxyC2STCP(setting.tcpConn, dataChan)
			proxyS2CTCP(setting.tcpConn, seq)
			return
		} else if setting.isAuthed && setting.isUDP && setting.success {
			go proxyC2SUDP(mgr, setting.udpListener, seq)
			proxyS2CUDP(mgr, setting.udpListener, seq)
			return
		} else {
			return
		}
	}
}

func (socks *Socks) buildConn(mgr *manager.Manager, setting *Setting, data []byte, seq uint64) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.SOCKSTCPDATA,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))), // No need to set route when agent send mess to admin
		Route:       protocol.TEMP_ROUTE,
	}

	failMess := &protocol.SocksTCPData{
		Seq:     seq,
		DataLen: uint64(len([]byte{0x05, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})),
		Data:    []byte{0x05, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}

	length := len(data)

	if length <= 2 {
		protocol.ConstructMessage(sMessage, header, failMess, false)
		sMessage.SendMessage()
		return
	}

	if data[0] == 0x05 {
		switch data[1] {
		case 0x01:
			tcpConnect(mgr, setting, data, seq, length)
		case 0x02:
			tcpBind(mgr, setting, data, seq, length)
		case 0x03:
			udpAssociate(mgr, setting, data, seq, length)
		default:
			protocol.ConstructMessage(sMessage, header, failMess, false)
			sMessage.SendMessage()
		}
	}
}

func DispathSocksMess(mgr *manager.Manager) {
	socks := newSocks()

	for {
		message := <-mgr.SocksManager.SocksMessChan

		switch mess := message.(type) {
		case *protocol.SocksStart:
			socks.Username = mess.Username
			socks.Password = mess.Password
			go socks.start(mgr)
		case *protocol.SocksTCPData:
			dataChan, exists := mgr.SocksManager.GetTCPDataChan(mess.Seq)

			dataChan <- mess.Data

			// if not exist
			if !exists {
				go socks.handleSocks(mgr, dataChan, mess.Seq)
			}
		case *protocol.SocksTCPFin:
			mgr.SocksManager.CloseTCP(mess.Seq)
		case *protocol.SocksUDPData:
			dataChan, _, ok := mgr.SocksManager.GetUDPChans(mess.Seq)
			if ok {
				dataChan <- mess.Data
			}
		case *protocol.UDPAssRes:
			_, readyChan, ok := mgr.SocksManager.GetUDPChans(mess.Seq)
			if ok {
				readyChan <- mess.Addr
			}
		}

	}
}
