package process

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"Shroud/agent/handler"
	"Shroud/agent/initial"
	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/share"
	"Shroud/share/transport"
	"Shroud/utils"
)

type Agent struct {
	UUID string
	Memo string

	options *initial.Options
	mgr     *manager.Manager

	childrenMessChan chan *ChildrenMess

	routeTable    map[string]string
	routeMu       sync.RWMutex
	lastHeartbeat int64
}

func (agent *Agent) updateRouteTable(table map[string]string) {
	agent.routeMu.Lock()
	agent.routeTable = table
	agent.routeMu.Unlock()
}

func (agent *Agent) lookupNextHop(destUUID string) (string, bool) {
	agent.routeMu.RLock()
	defer agent.routeMu.RUnlock()
	if agent.routeTable == nil {
		return "", false
	}
	nextHop, ok := agent.routeTable[destUUID]
	return nextHop, ok
}

type ChildrenMess struct {
	cHeader  *protocol.Header
	cMessage []byte
}

func NewAgent(options *initial.Options) *Agent {
	agent := new(Agent)
	agent.UUID = protocol.TEMP_UUID
	agent.childrenMessChan = make(chan *ChildrenMess, 5)
	agent.options = options
	return agent
}

func (agent *Agent) Run() {
	agent.sendMyInfo()
	// run manager
	agent.mgr = manager.NewManager()
	go agent.mgr.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// run dispatchers to dispatch all kinds of message
	go handler.DispatchListenMess(ctx, agent.mgr, agent.options)
	go handler.DispatchConnectMess(agent.mgr)
	go handler.DispathSocksMess(agent.mgr)
	go handler.DispatchForwardMess(agent.mgr)
	go handler.DispatchBackwardMess(ctx, agent.mgr)
	go handler.DispatchFileMess(agent.mgr)
	go handler.DispatchSSHMess(agent.mgr)
	go handler.DispatchSSHTunnelMess(agent.mgr)
	go handler.DispatchShellMess(agent.mgr, agent.options)
	go DispatchOfflineMess(agent)
	// run dispatcher to dispatch children's message
	go agent.dispatchChildrenMess()
	// waiting for child
	go agent.waitingChild()
	// heartbeat watchdog
	go agent.heartbeatWatchdog()
	// process data from upstream
	agent.handleDataFromUpstream()
	//agent.handleDataFromDownstream()
}

func (agent *Agent) sendMyInfo() {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)
	header := &protocol.Header{
		Sender:      agent.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.MYINFO,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))), // No need to set route when agent send mess to admin
		Route:       protocol.TEMP_ROUTE,
	}

	hostname, username := utils.GetSystemInfo()

	myInfoMess := &protocol.MyInfo{
		UUIDLen:     uint16(len(agent.UUID)),
		UUID:        agent.UUID,
		UsernameLen: uint64(len(username)),
		Username:    username,
		HostnameLen: uint64(len(hostname)),
		Hostname:    hostname,
		MemoLen:     uint64(len(agent.Memo)),
		Memo:        agent.Memo,
	}

	protocol.ConstructMessage(sMessage, header, myInfoMess, false)
	sMessage.SendMessage()
}

func (agent *Agent) handleDataFromUpstream() {
	rMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	for {
		header, message, err := protocol.DestructMessage(rMessage)
		if err != nil {
			select {
			case <-global.Session.TransportSwitch:
				rMessage = protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)
				continue
			default:
			}
			upstreamOffline(agent.mgr, agent.options)
			rMessage = protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)
			go agent.sendMyInfo()
			continue
		}

		if header.Accepter == agent.UUID {
			switch header.MessageType {
			case protocol.MYMEMO:
				mmsg, ok := message.(*protocol.MyMemo)
				if !ok {
					continue
				}
				agent.Memo = mmsg.Memo
			case protocol.SHELLREQ:
				fallthrough
			case protocol.SHELLCOMMAND:
				agent.mgr.ShellManager.ShellMessChan <- message
			case protocol.SSHREQ:
				fallthrough
			case protocol.SSHCOMMAND:
				agent.mgr.SSHManager.SSHMessChan <- message
			case protocol.SSHTUNNELREQ:
				agent.mgr.SSHTunnelManager.SSHTunnelMessChan <- message
			case protocol.FILESTATREQ:
				fallthrough
			case protocol.FILESTATRES:
				fallthrough
			case protocol.FILEDATA:
				fallthrough
			case protocol.FILEERR:
				fallthrough
			case protocol.FILEDOWNREQ:
				agent.mgr.FileManager.FileMessChan <- message
			case protocol.SOCKSSTART:
				fallthrough
			case protocol.SOCKSTCPDATA:
				fallthrough
			case protocol.SOCKSTCPFIN:
				fallthrough
			case protocol.UDPASSRES:
				fallthrough
			case protocol.SOCKSUDPDATA:
				agent.mgr.SocksManager.SocksMessChan <- message
			case protocol.FORWARDTEST:
				fallthrough
			case protocol.FORWARDSTART:
				fallthrough
			case protocol.FORWARDDATA:
				fallthrough
			case protocol.FORWARDFIN:
				agent.mgr.ForwardManager.ForwardMessChan <- message
			case protocol.BACKWARDTEST:
				fallthrough
			case protocol.BACKWARDSEQ:
				fallthrough
			case protocol.BACKWARDFIN:
				fallthrough
			case protocol.BACKWARDSTOP:
				fallthrough
			case protocol.BACKWARDDATA:
				agent.mgr.BackwardManager.BackwardMessChan <- message
			case protocol.CHILDUUIDRES:
				fallthrough
			case protocol.LISTENREQ:
				agent.mgr.ListenManager.ListenMessChan <- message
			case protocol.CONNECTSTART:
				agent.mgr.ConnectManager.ConnectMessChan <- message
			case protocol.UPSTREAMOFFLINE:
				fallthrough
			case protocol.UPSTREAMREONLINE:
				agent.mgr.OfflineManager.OfflineMessChan <- message
			case protocol.TRANSPORTSWITCHREQ:
				tsReq, ok := message.(*protocol.TransportSwitchReq)
				if !ok {
					continue
				}
				go agent.handleTransportSwitch(tsReq)
			case protocol.SHUTDOWN:
				cleanShutdown()
			case protocol.HEARTBEAT:
				hbMsg, ok := message.(*protocol.HeartbeatMsg)
				if !ok {
					continue
				}
				agent.handleHeartbeat(header, hbMsg)
			case protocol.ROUTETABLE:
				rtMsg, ok := message.(*protocol.RouteTableMsg)
				if !ok {
					continue
				}
				var table map[string]string
				if err := json.Unmarshal([]byte(rtMsg.Entries), &table); err == nil {
					agent.updateRouteTable(table)
				}
			default:
				log.Println("[*] Unknown Message!")
			}
		} else {
			raw, ok := message.([]byte)
			if !ok {
				continue
			}
			agent.childrenMessChan <- &ChildrenMess{
				cHeader:  header,
				cMessage: raw,
			}
		}
	}
}

func (agent *Agent) dispatchChildrenMess() {
	for {
		childrenMess := <-agent.childrenMessChan

		var childUUID string
		if nextHop, ok := agent.lookupNextHop(childrenMess.cHeader.Accepter); ok {
			childUUID = nextHop
		} else {
			childUUID = changeRoute(childrenMess.cHeader)
		}

		conn, ok := agent.mgr.ChildrenManager.GetConn(childUUID)
		if !ok {
			continue
		}

		childLinkKey, ok := agent.mgr.ChildrenManager.GetLinkKey(childUUID)
		if !ok {
			continue
		}

		sMessage := protocol.NewDownMsg(conn, global.G_Component.CryptoKey, childLinkKey, global.G_Component.UUID)

		protocol.ConstructMessage(sMessage, childrenMess.cHeader, childrenMess.cMessage, true)
		sMessage.SendMessage()
	}
}

func (agent *Agent) waitingChild() {
	for {
		childInfo := <-agent.mgr.ChildrenManager.ChildComeChan
		go agent.handleDataFromDownstream(childInfo.Conn, childInfo.UUID)
	}
}

func (agent *Agent) handleDataFromDownstream(conn net.Conn, uuid string) {
	childLinkKey, ok := agent.mgr.ChildrenManager.GetLinkKey(uuid)
	if !ok {
		return
	}
	rMessage := protocol.NewDownMsg(conn, global.G_Component.CryptoKey, childLinkKey, global.G_Component.UUID)

	for {
		header, message, err := protocol.DestructMessage(rMessage)
		if err != nil {
			downStreamOffline(agent.mgr, agent.options, uuid)
			return
		}

		sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

		protocol.ConstructMessage(sMessage, header, message, true)
		sMessage.SendMessage()
	}
}

func (agent *Agent) handleTransportSwitch(req *protocol.TransportSwitchReq) {
	listenAddr := global.G_Component.Conn.LocalAddr().String()
	host, _, _ := net.SplitHostPort(listenAddr)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = global.G_Component.Conn.LocalAddr().(*net.TCPAddr).IP.String()
	}

	listener, err := net.Listen("tcp", host+":0")
	if err != nil {
		agent.sendTransportSwitchRes(0, "")
		return
	}

	actualAddr := listener.Addr().String()
	if tcpL, ok := listener.(*net.TCPListener); ok {
		tcpL.SetDeadline(time.Now().Add(30 * time.Second))
	}

	agent.sendTransportSwitchRes(1, actualAddr)

	conn, err := listener.Accept()
	listener.Close()

	if err != nil {
		return
	}

	if global.Session.TLSEnable {
		tlsConfig, err := transport.NewServerTLSConfig()
		if err != nil {
			conn.Close()
			return
		}
		conn = transport.WrapTLSServerConn(conn, tlsConfig)
	}

	linkKey, _, err := share.PassiveAgentAuthAndExchange(conn, global.Session.AgentIdentity)
	if err != nil {
		conn.Close()
		return
	}

	rMessage := protocol.NewUpMsg(conn, global.G_Component.CryptoKey, linkKey, global.G_Component.UUID)
	fHeader, _, err := protocol.DestructMessage(rMessage)

	if err != nil {
		conn.Close()
		return
	}

	if fHeader.MessageType == protocol.TRANSPORTSWITCHDONE {
		if req.Method == 1 {
			global.SetTransportMode("tor")
		} else {
			global.SetTransportMode("raw")
		}
		oldConn := global.SwapGComponentConn(conn)
		global.Session.SetLinkKey(linkKey)
		global.SignalTransportSwitch()
		oldConn.Close()
		go agent.sendMyInfo()
	} else {
		conn.Close()
	}
}

func (agent *Agent) sendTransportSwitchRes(ok uint16, addr string) {
	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)
	header := &protocol.Header{
		Sender:      agent.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.TRANSPORTSWITCHRES,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}
	resMess := &protocol.TransportSwitchRes{
		OK:      ok,
		AddrLen: uint16(len(addr)),
		Addr:    addr,
	}
	protocol.ConstructMessage(sMessage, header, resMess, false)
	sMessage.SendMessage()
}

func changeRoute(header *protocol.Header) string {
	route := header.Route
	// find next uuid
	routes := strings.Split(route, ":")
	if len(routes) == 1 {
		header.Route = ""
		header.RouteLen = 0
		return routes[0]
	}

	header.Route = strings.Join(routes[1:], ":")
	header.RouteLen = uint32(len(header.Route))
	return routes[0]

}

func (agent *Agent) handleHeartbeat(header *protocol.Header, msg *protocol.HeartbeatMsg) {
	atomic.StoreInt64(&agent.lastHeartbeat, time.Now().Unix())

	sMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)
	ackHeader := &protocol.Header{
		Sender:      agent.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.HEARTBEATACK,
		RouteLen:    uint32(len(header.Route)),
		Route:       header.Route,
	}
	ackMess := &protocol.HeartbeatAckMsg{Seq: msg.Seq}
	protocol.ConstructMessage(sMessage, ackHeader, ackMess, false)
	sMessage.SendMessage()
}

func (agent *Agent) heartbeatWatchdog() {
	for {
		time.Sleep(30 * time.Second)
		last := atomic.LoadInt64(&agent.lastHeartbeat)
		if last == 0 {
			continue
		}
		if time.Now().Unix()-last > 90 {
			cleanShutdown()
		}
	}
}
