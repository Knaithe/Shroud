package handler

import (
	"Shroud/admin/manager"
	"Shroud/admin/printer"
	"Shroud/admin/topology"
	"Shroud/global"
	"Shroud/protocol"
	"Shroud/utils"
)

const (
	NORMAL = iota
	IPTABLES
	SOREUSE
	TORHIDDEN
)

type Listen struct {
	Method int
	Addr   string
}

func NewListen() *Listen {
	return new(Listen)
}

func (listen *Listen) LetListen(mgr *manager.Manager, route, uuid string) error {
	var finalAddr string

	if listen.Method == NORMAL {
		var err error
		finalAddr, _, err = utils.CheckIPPort(listen.Addr)
		if err != nil {
			return err
		}
	}

	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    uuid,
		MessageType: protocol.LISTENREQ,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	listenReqMess := &protocol.ListenReq{
		Method:  uint16(listen.Method),
		AddrLen: uint64(len(finalAddr)),
		Addr:    finalAddr,
	}

	protocol.ConstructMessage(sMessage, header, listenReqMess, false)
	sMessage.SendMessage()

	if <-mgr.ListenManager.ListenReady {
		if listen.Method == NORMAL {
			printer.Success("\r\n[*] Node is listening on %s", listen.Addr)
		} else if listen.Method == TORHIDDEN {
			printer.Success("\r\n[*] Node has created Tor hidden service, waiting for child....")
		} else {
			printer.Success("\r\n[*] Node is reusing port successfully,just waiting for child....")
		}
	} else {
		if listen.Method == NORMAL {
			printer.Success("\r\n[*] Node cannot listen on %s", listen.Addr)
		} else if listen.Method == TORHIDDEN {
			printer.Fail("\r\n[*] Node failed to create Tor hidden service!")
		} else {
			printer.Success("\r\n[*] Node cannot reuse port, please check if node is initialized via reusing!")
		}
	}

	return nil
}

// this function is SPECIAL,handling childuuidreq from both "listen" && "node reuse" && "connect" && "sshtunnel" condition
func dispatchChildUUID(mgr *manager.Manager, topo *topology.Topology, req *protocol.ChildUUIDReq) {
	uuid := utils.GenerateUUID()
	node := topology.NewNode(uuid, req.IP)
	topoTask := &topology.TopoTask{
		Mode:       topology.ADDNODE,
		Target:     node,
		ParentUUID: req.ParentUUID,
		IsFirst:    false,
	}
	topo.TaskChan <- topoTask
	topoResult := <-topo.ResultChan
	childIDNum := topoResult.IDNum

	topoTask = &topology.TopoTask{
		Mode: topology.CALCULATE,
	}
	topo.TaskChan <- topoTask
	<-topo.ResultChan
	DistributeRouteTables(topo)

	topoTask = &topology.TopoTask{
		Mode: topology.GETROUTE,
		UUID: req.ParentUUID,
	}
	topo.TaskChan <- topoTask
	topoResult = <-topo.ResultChan
	route := topoResult.Route

	sMessage := protocol.NewDownMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.LinkKey, global.G_Component.UUID)

	header := &protocol.Header{
		Sender:      protocol.ADMIN_UUID,
		Accepter:    req.ParentUUID,
		MessageType: protocol.CHILDUUIDRES,
		RouteLen:    uint32(len([]byte(route))),
		Route:       route,
	}

	cUUIDResMess := &protocol.ChildUUIDRes{
		UUIDLen: uint16(len(uuid)),
		UUID:    uuid,
		OK:      1,
	}

	if global.Session.AdminIdentity == nil {
		cUUIDResMess.OK = 0
		cUUIDResMess.Error = "admin identity not initialized"
		cUUIDResMess.ErrorLen = uint16(len(cUUIDResMess.Error))
	}

	if cUUIDResMess.OK == 1 && req.WantsEnrollment == 1 {
		resp, err := global.Session.AdminIdentity.IssueAgentCertificate(req.Ed25519Public, req.X25519Public)
		if err != nil {
			cUUIDResMess.OK = 0
			cUUIDResMess.Error = err.Error()
			cUUIDResMess.ErrorLen = uint16(len(cUUIDResMess.Error))
		} else {
			cUUIDResMess.EnrollmentResponse = resp
			cUUIDResMess.EnrollmentResponseLen = uint32(len(resp.AgentCert.Signature))
			_ = global.Session.AdminIdentity.BindProtocolUUID(uuid, resp.AgentCert)
		}
	} else if cUUIDResMess.OK == 1 && len(req.Cert.Signature) != 0 {
		if err := global.Session.AdminIdentity.VerifyPeerCertificate(req.Cert); err != nil {
			cUUIDResMess.OK = 0
			cUUIDResMess.Error = err.Error()
			cUUIDResMess.ErrorLen = uint16(len(cUUIDResMess.Error))
		} else {
			_ = global.Session.AdminIdentity.BindProtocolUUID(uuid, req.Cert)
		}
	}

	protocol.ConstructMessage(sMessage, header, cUUIDResMess, false)
	sMessage.SendMessage()

	printer.Success("\r\n[*] New node online! Node id is %d\r\n", childIDNum)
}

func DispatchListenMess(mgr *manager.Manager, topo *topology.Topology) {
	for {
		message := <-mgr.ListenManager.ListenMessChan

		switch mess := message.(type) {
		case *protocol.ListenRes:
			if mess.OK == 1 {
				mgr.ListenManager.ListenReady <- true
			} else {
				mgr.ListenManager.ListenReady <- false
			}
		case *protocol.ChildUUIDReq:
			go dispatchChildUUID(mgr, topo, mess)
		}
	}
}
