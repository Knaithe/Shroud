package handler

import (
	"Shroud/global"
	"Shroud/protocol"
)

func (socks *Socks) checkMethod(setting *Setting, data []byte, seq uint64) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.SOCKSTCPDATA,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))), // No need to set route when agent send mess to admin
		Route:       protocol.TEMP_ROUTE,
	}

	failMess := &protocol.SocksTCPData{
		Seq:     seq,
		DataLen: uint64(len([]byte{0x05, 0xff})),
		Data:    []byte{0x05, 0xff},
	}

	noneMess := &protocol.SocksTCPData{
		Seq:     seq,
		DataLen: uint64(len([]byte{0x05, 0x00})),
		Data:    []byte{0x05, 0x00},
	}

	passMess := &protocol.SocksTCPData{
		Seq:     seq,
		DataLen: uint64(len([]byte{0x05, 0x02})),
		Data:    []byte{0x05, 0x02},
	}

	// avoid the scenario that we can get full socks protocol header (rarely happen,just in case)
	defer func() {
		if r := recover(); r != nil {
			setting.method = "ILLEGAL"
		}
	}()

	if data[0] == 0x05 {
		nMethods := int(data[1])

		var supportMethodFinded, userPassFinded, noAuthFinded bool

		for _, method := range data[2 : 2+nMethods] {
			if method == 0x00 {
				noAuthFinded = true
				supportMethodFinded = true
			} else if method == 0x02 {
				userPassFinded = true
				supportMethodFinded = true
			}
		}

		if !supportMethodFinded {
			protocol.ConstructMessage(sMessage, header, failMess, false)
			sMessage.SendMessage()
			setting.method = "ILLEGAL"
			return
		}

		if noAuthFinded && (socks.Username == "" && socks.Password == "") {
			protocol.ConstructMessage(sMessage, header, noneMess, false)
			sMessage.SendMessage()
			setting.method = "NONE"
			setting.isAuthed = true
			return
		} else if userPassFinded && (socks.Username != "" && socks.Password != "") {
			protocol.ConstructMessage(sMessage, header, passMess, false)
			sMessage.SendMessage()
			setting.method = "PASSWORD"
			return
		} else {
			protocol.ConstructMessage(sMessage, header, failMess, false)
			sMessage.SendMessage()
			setting.method = "ILLEGAL"
			return
		}
	}
	// send nothing
	setting.method = "ILLEGAL"
}

func (socks *Socks) auth(setting *Setting, data []byte, seq uint64) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.SOCKSTCPDATA,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	failMess := &protocol.SocksTCPData{
		Seq:     seq,
		DataLen: uint64(len([]byte{0x01, 0x01})),
		Data:    []byte{0x01, 0x01},
	}

	succMess := &protocol.SocksTCPData{
		Seq:     seq,
		DataLen: uint64(len([]byte{0x01, 0x00})),
		Data:    []byte{0x01, 0x00},
	}

	defer func() {
		if r := recover(); r != nil {
			setting.isAuthed = false
		}
	}()

	ulen := int(data[1])
	slen := int(data[2+ulen])
	clientName := string(data[2 : 2+ulen])
	clientPass := string(data[3+ulen : 3+ulen+slen])

	if clientName != socks.Username || clientPass != socks.Password {
		protocol.ConstructMessage(sMessage, header, failMess, false)
		sMessage.SendMessage()
		setting.isAuthed = false
		return
	}
	// username && password all fits!
	protocol.ConstructMessage(sMessage, header, succMess, false)
	sMessage.SendMessage()
	setting.isAuthed = true
}
