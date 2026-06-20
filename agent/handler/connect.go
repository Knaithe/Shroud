package handler

import (
	"crypto/tls"
	"errors"
	"net"
	"time"

	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/identity"
	"Shroud/protocol"
	"Shroud/share"
	"Shroud/share/transport"
)

type Connect struct {
	Addr string
}

func newConnect(addr string) *Connect {
	connect := new(Connect)
	connect.Addr = addr
	return connect
}

func (connect *Connect) start(mgr *manager.Manager) {
	var sUMessage, sLMessage, rMessage protocol.Message

	sUMessage = protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	hiHeader := &protocol.Header{
		Sender:      protocol.ADMIN_UUID, // fake admin
		Accepter:    protocol.TEMP_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	// fake admin
	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len("Shhh...")),
		Greeting:    "Shhh...",
		UUIDLen:     uint16(len(protocol.ADMIN_UUID)),
		UUID:        protocol.ADMIN_UUID,
		IsAdmin:     1,
		IsReconnect: 0,
	}

	doneHeader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.CONNECTDONE,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	doneSuccMess := &protocol.ConnectDone{
		OK: 1,
	}

	doneFailMess := &protocol.ConnectDone{
		OK: 0,
	}

	var (
		conn net.Conn
		err  error
	)

	defer func() {
		if err != nil {
			protocol.ConstructMessage(sUMessage, doneHeader, doneFailMess, false)
			sUMessage.SendMessage()
		}
	}()

	dial := func() (net.Conn, error) {
		var c net.Conn
		var dialErr error
		if share.IsOnionAddress(connect.Addr) {
			torSocks := global.Session.TorProxy
			if torSocks == "" {
				torSocks = "127.0.0.1:9050"
			}
			torProxy := share.NewTorProxy(connect.Addr, torSocks)
			c, dialErr = torProxy.Dial()
		} else {
			c, dialErr = net.DialTimeout("tcp", connect.Addr, 10*time.Second)
		}
		if dialErr != nil {
			return nil, dialErr
		}
		if global.Session.TLSEnable {
			var tlsConfig *tls.Config
			tlsConfig, dialErr = transport.NewClientTLSConfig("", global.Session.TLSFingerprint, global.Session.TLSInsecure)
			if dialErr != nil {
				c.Close()
				return nil, dialErr
			}
			c = transport.WrapTLSClientConn(c, tlsConfig)
		}
		param := new(protocol.NegParam)
		param.Conn = c
		proto := protocol.NewDownProto(param)
		if dialErr = proto.CNegotiate(); dialErr != nil {
			c.Close()
			return nil, dialErr
		}
		return c, nil
	}

	var linkKey []byte
	var peerCert identity.Certificate
	var preassignedUUID string
	conn, linkKey, peerCert, preassignedUUID, err = activeChildAuthWithRetry(dial, mgr)
	if err != nil {
		return
	}

	sLMessage = protocol.NewDownMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.ADMIN_UUID)

	protocol.ConstructMessage(sLMessage, hiHeader, hiMess, false)
	sLMessage.SendMessage()

	rMessage = protocol.NewDownMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.ADMIN_UUID)
	fHeader, fMessage, err := protocol.DestructMessage(rMessage)

	if err != nil {
		conn.Close()
		return
	}

	var childUUID string

	if fHeader.MessageType == protocol.HI {
		mmess := fMessage.(*protocol.HIMess)
		if mmess.Greeting == "Keep silent" && mmess.IsAdmin == 0 {
			if mmess.IsReconnect == 0 {
				childIP := conn.RemoteAddr().String()

				if preassignedUUID != "" {
					childUUID = preassignedUUID
				} else {
					res, reqErr := requestChildUUID(mgr, childIP, peerCert, false, nil, nil)
					if reqErr != nil {
						err = reqErr
						conn.Close()
						return
					}
					childUUID = res.UUID
				}

				uuidHeader := &protocol.Header{
					Sender:      protocol.ADMIN_UUID, // Fake admin LOL
					Accepter:    protocol.TEMP_UUID,
					MessageType: protocol.UUID,
					RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
					Route:       protocol.TEMP_ROUTE,
				}

				uuidMess := &protocol.UUIDMess{
					UUIDLen: uint16(len(childUUID)),
					UUID:    childUUID,
				}

				protocol.ConstructMessage(sLMessage, uuidHeader, uuidMess, false)
				sLMessage.SendMessage()
			} else {
				reheader := &protocol.Header{
					Sender:      global.G_Component.UUID,
					Accepter:    protocol.ADMIN_UUID,
					MessageType: protocol.NODEREONLINE,
					RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
					Route:       protocol.TEMP_ROUTE,
				}

				reMess := &protocol.NodeReonline{
					ParentUUIDLen: uint16(len(global.G_Component.UUID)),
					ParentUUID:    global.G_Component.UUID,
					UUIDLen:       uint16(len(mmess.UUID)),
					UUID:          mmess.UUID,
					IPLen:         uint16(len(conn.RemoteAddr().String())),
					IP:            conn.RemoteAddr().String(),
				}

				protocol.ConstructMessage(sUMessage, reheader, reMess, false)
				sUMessage.SendMessage()

				childUUID = mmess.UUID
			}

			mgr.ChildrenManager.NewChild(childUUID, conn, linkKey)

			mgr.ChildrenManager.ChildComeChan <- &manager.ChildInfo{UUID: childUUID, Conn: conn}

			protocol.ConstructMessage(sUMessage, doneHeader, doneSuccMess, false)
			sUMessage.SendMessage()

			return
		}
	}

	conn.Close()
	err = errors.New("node looks invalid")
}

func DispatchConnectMess(mgr *manager.Manager) {
	for {
		message := <-mgr.ConnectManager.ConnectMessChan

		switch mess := message.(type) {
		case *protocol.ConnectStart:
			connect := newConnect(mess.Addr)
			go connect.start(mgr)
		}
	}
}
