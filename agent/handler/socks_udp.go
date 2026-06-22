package handler

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/utils"
)

type socksLocalAddr struct {
	Host string
	Port int
}

func (addr *socksLocalAddr) byteArray() []byte {
	bytes := make([]byte, 6)
	copy(bytes[:4], net.ParseIP(addr.Host).To4())
	bytes[4] = byte(addr.Port >> 8)
	bytes[5] = byte(addr.Port % 256)
	return bytes
}

// Based on rfc1928,agent must send message strictly
// UDPAssociate UDPAssociate方式
func udpAssociate(mgr *manager.Manager, setting *Setting, data []byte, seq uint64, length int) {
	setting.isUDP = true

	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	dataHeader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.SOCKSTCPDATA,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	assHeader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.UDPASSSTART,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	failMess := &protocol.SocksTCPData{
		Seq:     seq,
		DataLen: uint64(len([]byte{0x05, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})),
		Data:    []byte{0x05, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}

	defer func() {
		if r := recover(); r != nil {
			setting.success = false
		}
	}()

	var host string
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
		protocol.ConstructMessage(sMessage, dataHeader, failMess, false)
		sMessage.SendMessage()
		setting.success = false
		return
	}

	port := utils.Int2Str(int(data[length-2])<<8 | int(data[length-1]))

	udpListenerAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:0")
	if err != nil {
		protocol.ConstructMessage(sMessage, dataHeader, failMess, false)
		sMessage.SendMessage()
		setting.success = false
		return
	}

	udpListener, err := net.ListenUDP("udp", udpListenerAddr)
	if err != nil {
		protocol.ConstructMessage(sMessage, dataHeader, failMess, false)
		sMessage.SendMessage()
		setting.success = false
		return
	}

	sourceAddr := net.JoinHostPort(host, port)

	if !mgr.SocksManager.CheckUDP(seq) {
		udpListener.Close()
		protocol.ConstructMessage(sMessage, dataHeader, failMess, false)
		sMessage.SendMessage()
		setting.success = false
		return
	}

	_, readyChan, ok := mgr.SocksManager.GetUDPChans(seq)
	if !ok {
		protocol.ConstructMessage(sMessage, dataHeader, failMess, false)
		sMessage.SendMessage()
		setting.success = false
		return
	}

	assMess := &protocol.UDPAssStart{
		Seq:           seq,
		SourceAddrLen: uint16(len([]byte(sourceAddr))),
		SourceAddr:    sourceAddr,
	}

	protocol.ConstructMessage(sMessage, assHeader, assMess, false)
	sMessage.SendMessage()

	if adminResponse, ok := <-readyChan; adminResponse != "" && ok {
		temp := strings.Split(adminResponse, ":")
		adminAddr := temp[0]
		adminPort, _ := strconv.Atoi(temp[1])

		localAddr := socksLocalAddr{adminAddr, adminPort}
		buf := make([]byte, 10)
		copy(buf, []byte{0x05, 0x00, 0x00, 0x01})
		copy(buf[4:], localAddr.byteArray())

		dataMess := &protocol.SocksTCPData{
			Seq:     seq,
			DataLen: 10,
			Data:    buf,
		}

		protocol.ConstructMessage(sMessage, dataHeader, dataMess, false)
		sMessage.SendMessage()

		setting.udpListener = udpListener
		setting.success = true
		return
	}

	protocol.ConstructMessage(sMessage, dataHeader, failMess, false)
	sMessage.SendMessage()
	setting.success = false
}

// proxyC2SUDP
func proxyC2SUDP(mgr *manager.Manager, listener *net.UDPConn, seq uint64) {
	dataChan, _, ok := mgr.SocksManager.GetUDPChans(seq)
	// no need to check if OK,cuz if not,"data, ok := <-dataChan" will help us to exit
	_ = ok

	defer func() {
		// Just avoid panic
		if r := recover(); r != nil {
			go func() { //continue to read channel,avoid some remaining data sent by admin blocking our dispatcher
				for {
					_, ok := <-dataChan
					if !ok {
						return
					}
				}
			}()
		}
	}()

	for {
		var remote string
		var udpData []byte

		data, ok := <-dataChan
		if !ok {
			listener.Close()
			return
		}

		buf := []byte(data)

		if buf[0] != 0x00 || buf[1] != 0x00 || buf[2] != 0x00 {
			continue
		}

		udpHeader := make([]byte, 0, 1024)
		addrtype := buf[3]

		if addrtype == 0x01 { //IPV4
			ip := net.IPv4(buf[4], buf[5], buf[6], buf[7])
			remote = fmt.Sprintf("%s:%d", ip.String(), uint(buf[8])<<8+uint(buf[9]))
			udpData = buf[10:]
			udpHeader = append(udpHeader, buf[:10]...)
		} else if addrtype == 0x03 { //DOMAIN
			nmlen := int(buf[4])
			nmbuf := buf[5 : 5+nmlen+2]
			remote = fmt.Sprintf("%s:%d", nmbuf[:nmlen], uint(nmbuf[nmlen])<<8+uint(nmbuf[nmlen+1]))
			udpData = buf[8+nmlen:]
			udpHeader = append(udpHeader, buf[:8+nmlen]...)
		} else if addrtype == 0x04 { //IPV6
			ip := net.IP{buf[4], buf[5], buf[6], buf[7],
				buf[8], buf[9], buf[10], buf[11], buf[12],
				buf[13], buf[14], buf[15], buf[16], buf[17],
				buf[18], buf[19]}
			remote = fmt.Sprintf("[%s]:%d", ip.String(), uint(buf[20])<<8+uint(buf[21]))
			udpData = buf[22:]
			udpHeader = append(udpHeader, buf[:22]...)
		} else {
			continue
		}

		remoteAddr, err := net.ResolveUDPAddr("udp", remote)
		if err != nil {
			continue
		}

		mgr.SocksManager.UpdateUDPHeader(seq, remote, udpHeader)

		listener.WriteToUDP(udpData, remoteAddr)
	}
}

// proxyS2CUDP
func proxyS2CUDP(mgr *manager.Manager, listener *net.UDPConn, seq uint64) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.SOCKSUDPDATA,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	buffer := make([]byte, 20480)
	var data []byte
	var finalLength int

	for {
		length, addr, err := listener.ReadFromUDP(buffer)
		if err != nil {
			listener.Close()
			return
		}

		udpHeader, ok := mgr.SocksManager.GetUDPHeader(seq, addr.String())
		if ok {
			finalLength = len(udpHeader) + length
			data = make([]byte, 0, finalLength)
			data = append(data, udpHeader...)
			data = append(data, buffer[:length]...)
		} else {
			return
		}

		dataMess := &protocol.SocksUDPData{
			Seq:     seq,
			DataLen: uint64(finalLength),
			Data:    data,
		}

		protocol.ConstructMessage(sMessage, header, dataMess, false)
		sMessage.SendMessage()
	}
}
