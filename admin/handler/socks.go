package handler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"Shroud/admin/manager"
	"Shroud/admin/topology"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/utils"
)

type Socks struct {
	Username string
	Password string
	Addr     string
	Port     string
}

func NewSocks(param string) *Socks {
	socks := new(Socks)

	slice := strings.SplitN(param, ":", 2)

	if len(slice) < 2 {
		socks.Addr = "127.0.0.1"
		socks.Port = param
	} else {
		socks.Addr = slice[0]
		socks.Port = slice[1]
	}

	return socks
}

func (socks *Socks) LetSocks(ctx context.Context, mgr *manager.Manager, route string, uuid string) error {
	var addr string

	addr = fmt.Sprintf("%s:%s", socks.Addr, socks.Port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	// register a new SOCKS service
	if !mgr.SocksManager.NewSocks(uuid, socks.Addr, socks.Port, socks.Username, socks.Password, listener) {
		// node and socks service must be one-to-one
		err := errors.New("SOCKS is already running on the current node. Use 'stopsocks' to stop the existing one.")
		listener.Close()
		return err
	}

	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.SOCKSSTART,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	socksStartMess := &protocol.SocksStart{
		UsernameLen: uint64(len([]byte(socks.Username))),
		Username:    socks.Username,
		PasswordLen: uint64(len([]byte(socks.Password))),
		Password:    socks.Password,
	}

	protocol.ConstructMessage(sMessage, header, socksStartMess, false)
	sMessage.SendMessage()

	if ready := <-mgr.SocksManager.SocksReady; !ready {
		err := errors.New("failed to start socks. If you just stopped the socks service, please wait for cleanup to finish.")
		StopSocks(mgr, uuid)
		return err
	}

	go socks.handleSocksListener(ctx, mgr, listener, route, uuid)

	return nil
}

func (socks *Socks) handleSocksListener(ctx context.Context, mgr *manager.Manager, listener net.Listener, route string, uuid string) {
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

		// ask new seq num
		seq := mgr.SocksManager.GetSocksSeq(uuid)

		// save the socket
		if !mgr.SocksManager.AddTCPSocket(uuid, seq, conn) {
			conn.Close()
			return
		}

		// handle it!
		go socks.handleSocks(mgr, conn, route, uuid, seq)
	}
}

func (socks *Socks) handleSocks(mgr *manager.Manager, conn net.Conn, route string, uuid string, seq uint64) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.SOCKSTCPDATA,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	tcpDataChan, ok := mgr.SocksManager.GetTCPDataChan(uuid, seq)
	if !ok {
		return
	}

	// handle data from dispatcher
	go func() {
		for {
			if data, ok := <-tcpDataChan; ok {
				conn.Write(data)
			} else {
				conn.Close()
				return
			}
		}
	}()

	var sendSth bool

	// SendTCPFin after browser close the conn
	defer func() {
		// check if "sendSth" is true
		// if true, then tell agent that the conn is closed
		// but keep "handle received data" working to achieve socksdata from agent that still on the way
		// if false, don't tell agent and do cleanup alone
		if !sendSth {
			// call HandleTCPFin by myself
			mgr.SocksManager.CloseTCP(seq)
			return
		}

		finHeader := &protocol.Header{
			Sender:      protocol.ADMIN_UUID,
			Accepter:    uuid,
			MessageType: protocol.SOCKSTCPFIN,
			RouteLen:    uint32(len([]byte(route))),
			Route:       route,
		}
		finMess := &protocol.SocksTCPFin{
			Seq: seq,
		}

		protocol.ConstructMessage(sMessage, finHeader, finMess, false)
		sMessage.SendMessage()
	}()

	// handling data that comes from browser
	buffer := make([]byte, 20480)

	// try to receive first packet
	// avoid browser to close the conn but sends nothing
	length, err := conn.Read(buffer)
	if err != nil {
		conn.Close() // close conn immediately
		return
	}

	socksDataMess := &protocol.SocksTCPData{
		Seq:     seq,
		DataLen: uint64(length),
		Data:    buffer[:length],
	}

	protocol.ConstructMessage(sMessage, header, socksDataMess, false)
	sMessage.SendMessage()

	// browser sends sth, so handling conn normally and setting sendSth->true
	for {
		length, err := conn.Read(buffer)
		if err != nil {
			sendSth = true
			conn.Close() // close conn immediately,in case of sth is sent after TCPFin
			return
		}

		socksDataMess := &protocol.SocksTCPData{
			Seq:     seq,
			DataLen: uint64(length),
			Data:    buffer[:length],
		}

		protocol.ConstructMessage(sMessage, header, socksDataMess, false)
		sMessage.SendMessage()
	}
}

func startUDPAss(mgr *manager.Manager, topo *topology.Topology, seq uint64) {
	var (
		err             error
		udpListenerAddr *net.UDPAddr
		udpListener     *net.UDPConn
	)

	tcpAddr, uuid, startOK := mgr.SocksManager.GetUDPStartInfo(seq)

	topoTask := &topology.TopoTask{
		Mode: topology.GETROUTE,
		UUID: uuid,
	}
	topo.TaskChan <- topoTask
	topoResult := <-topo.ResultChan
	route := topoResult.Route

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.UDPASSRES,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	failMess := &protocol.UDPAssRes{
		Seq: seq,
		OK:  0,
	}

	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	defer func() {
		if err != nil {
			protocol.ConstructMessage(sMessage, header, failMess, false)
			sMessage.SendMessage()
		}
	}()

	if startOK {
		udpListenerAddr, err = net.ResolveUDPAddr("udp", tcpAddr+":0")
		if err != nil {
			return
		}

		udpListener, err = net.ListenUDP("udp", udpListenerAddr)
		if err != nil {
			return
		}

		if !mgr.SocksManager.UpdateUDP(uuid, seq, udpListener) {
			err = errors.New("TCP conn seems disconnected")
			return
		}

		go handleUDPAss(mgr, udpListener, route, uuid, seq)

		succMess := &protocol.UDPAssRes{
			Seq:     seq,
			OK:      1,
			AddrLen: uint16(len(udpListener.LocalAddr().String())),
			Addr:    udpListener.LocalAddr().String(),
		}

		protocol.ConstructMessage(sMessage, header, succMess, false)
		sMessage.SendMessage()
	} else {
		err = errors.New("TCP conn seems disconnected")
		return
	}
}

func handleUDPAss(mgr *manager.Manager, listener *net.UDPConn, route string, uuid string, seq uint64) {
	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	dataHeader := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.SOCKSUDPDATA,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	udpDataChan, ok := mgr.SocksManager.GetUDPDataChan(uuid, seq)
	if !ok {
		return
	}

	buffer := make([]byte, 20480)

	var alreadyGetAddr bool
	for {
		length, addr, err := listener.ReadFromUDP(buffer)
		if !alreadyGetAddr {
			go func() {
				for {
					if data, ok := <-udpDataChan; ok {
						listener.WriteToUDP(data, addr)
					} else {
						listener.Close()
						return
					}
				}
			}()
			alreadyGetAddr = true
		}

		if err != nil {
			listener.Close()
			return
		}

		udpDataMess := &protocol.SocksUDPData{
			Seq:     seq,
			DataLen: uint64(length),
			Data:    buffer[:length],
		}

		protocol.ConstructMessage(sMessage, dataHeader, udpDataMess, false)
		sMessage.SendMessage()
	}
}

func GetSocksInfo(mgr *manager.Manager, uuid string) bool {
	info, ok := mgr.SocksManager.GetSocksInfo(uuid)

	if ok {
		fmt.Printf(
			"\r\nSocks Info ---> ListenAddr: %s:%s    Username: %s   Password: %s",
			info.Addr,
			info.Port,
			info.Username,
			info.Password,
		)
	}

	return ok
}

func StopSocks(mgr *manager.Manager, uuid string) {
	mgr.SocksManager.CloseSocks(uuid)
}

func DispathSocksMess(mgr *manager.Manager, topo *topology.Topology) {
	for {
		message := <-mgr.SocksManager.SocksMessChan

		switch mess := message.(type) {
		case *protocol.SocksReady:
			if mess.OK == 1 {
				mgr.SocksManager.SocksReady <- true
			} else {
				mgr.SocksManager.SocksReady <- false
			}
		case *protocol.SocksTCPData:
			ch, ok := mgr.SocksManager.GetTCPDataChanBySeq(mess.Seq)
			if ok {
				utils.SafeSend(ch, mess.Data)
			}
		case *protocol.SocksTCPFin:
			mgr.SocksManager.CloseTCP(mess.Seq)
		case *protocol.UDPAssStart:
			go startUDPAss(mgr, topo, mess.Seq)
		case *protocol.SocksUDPData:
			ch, ok := mgr.SocksManager.GetUDPDataChanBySeq(mess.Seq)
			if ok {
				utils.SafeSend(ch, mess.Data)
			}
		}
	}
}
