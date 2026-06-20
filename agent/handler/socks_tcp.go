package handler

import (
	"fmt"
	"net"
	"time"

	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/utils"
)

// TCPConnect
func tcpConnect(mgr *manager.Manager, setting *Setting, data []byte, seq uint64, length int) {
	var host string
	var err error

	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.SOCKSTCPDATA,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	failMess := &protocol.SocksTCPData{
		Seq:     seq,
		DataLen: uint64(len([]byte{0x05, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})),
		Data:    []byte{0x05, 0x04, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}

	succMess := &protocol.SocksTCPData{
		Seq:     seq,
		DataLen: uint64(len([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})),
		Data:    []byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}

	defer func() {
		if r := recover(); r != nil {
			setting.tcpConnected = false
		}
	}()

	switch data[3] {
	case 0x01:
		host = net.IPv4(data[4], data[5], data[6], data[7]).String()
	case 0x03:
		host = string(data[5 : length-2])
	case 0x04:
		host = net.IP{data[4], data[5], data[6], data[7],
			data[8], data[9], data[10], data[11], data[12],
			data[13], data[14], data[15], data[16], data[17],
			data[18], data[19]}.String()
	default:
		protocol.ConstructMessage(sMessage, header, failMess, false)
		sMessage.SendMessage()
		setting.tcpConnected = false
		return
	}

	port := utils.Int2Str(int(data[length-2])<<8 | int(data[length-1]))

	setting.tcpConn, err = net.DialTimeout("tcp", net.JoinHostPort(host, port), 10*time.Second)

	if err != nil {
		protocol.ConstructMessage(sMessage, header, failMess, false)
		sMessage.SendMessage()
		setting.tcpConnected = false
		return
	}

	if !mgr.SocksManager.CheckTCP(seq) {
		setting.tcpConn.Close()
		protocol.ConstructMessage(sMessage, header, failMess, false)
		sMessage.SendMessage()
		setting.tcpConnected = false
		return
	}

	protocol.ConstructMessage(sMessage, header, succMess, false)
	sMessage.SendMessage()
	setting.tcpConnected = true
}

func proxyC2STCP(conn net.Conn, dataChan chan []byte) {
	for {
		data, ok := <-dataChan
		if !ok { // no need to send FIN actively
			conn.Close()
			return
		}
		conn.Write(data)
	}
}

func proxyS2CTCP(conn net.Conn, seq uint64) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.SOCKSTCPDATA,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))), // No need to set route when agent send mess to admin
		Route:       protocol.TEMP_ROUTE,
	}

	buffer := make([]byte, 20480)
	for {
		length, err := conn.Read(buffer)
		if err != nil {
			conn.Close() // close conn immediately
			return
		}

		dataMess := &protocol.SocksTCPData{
			Seq:     seq,
			DataLen: uint64(length),
			Data:    buffer[:length],
		}

		protocol.ConstructMessage(sMessage, header, dataMess, false)
		sMessage.SendMessage()
	}
}

// TCPBind TCPBind方式
func tcpBind(mgr *manager.Manager, setting *Setting, data []byte, seq uint64, length int) {
	fmt.Println("Not ready") //limited use, add to Todo
	setting.tcpConnected = false
}
