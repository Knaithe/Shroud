package handler

import (
	"errors"
	"log"
	"net"
	"time"

	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/identity"
	"Shroud/protocol"
	"Shroud/share"
)

type negotiatedDial func() (net.Conn, error)

func activeChildAuthWithRetry(dial negotiatedDial, mgr *manager.Manager) (net.Conn, []byte, identity.Certificate, string, error) {
	agentID := global.Session.AgentIdentity
	if agentID == nil || !agentID.HasCertificate() {
		return nil, nil, identity.Certificate{}, "", errors.New("local agent identity is not enrolled")
	}
	conn, err := dial()
	if err != nil {
		return nil, nil, identity.Certificate{}, "", err
	}
	linkKey, peerCert, err := share.ActiveAgentCertAuthAndExchange(conn, agentID)
	if err == nil {
		return conn, linkKey, peerCert, "", nil
	}
	if len(share.AuthKey) == 0 || !errors.Is(err, share.ErrPeerNoCert) {
		conn.Close()
		return nil, nil, identity.Certificate{}, "", err
	}
	conn.Close()

	log.Printf("[*] WARNING: peer does not support cert auth, falling back to token enrollment: %v", err)

	conn, err = dial()
	if err != nil {
		return nil, nil, identity.Certificate{}, "", err
	}
	childIP := conn.RemoteAddr().String()
	var assignedUUID string
	linkKey, peerCert, err = share.ActiveAgentRelayEnrollAndExchange(conn, agentID, relayChildEnrollment(mgr, childIP, &assignedUUID))
	if err != nil {
		conn.Close()
		return nil, nil, identity.Certificate{}, "", err
	}
	return conn, linkKey, peerCert, assignedUUID, nil
}

func passiveChildAuth(conn net.Conn, mgr *manager.Manager, childIP string) ([]byte, identity.Certificate, string, error) {
	agentID := global.Session.AgentIdentity
	if agentID == nil || !agentID.HasCertificate() {
		conn.Close()
		return nil, identity.Certificate{}, "", errors.New("local agent identity is not enrolled")
	}
	var assignedUUID string
	linkKey, peerCert, err := share.PassiveAgentEnrollRelayOrCertAndExchange(conn, agentID, relayChildEnrollment(mgr, childIP, &assignedUUID))
	if err != nil {
		return nil, identity.Certificate{}, "", err
	}
	return linkKey, peerCert, assignedUUID, nil
}

func soReusePassiveChildAuth(conn net.Conn, reusePort string, mgr *manager.Manager, childIP string) ([]byte, identity.Certificate, string, error) {
	agentID := global.Session.AgentIdentity
	if agentID == nil || !agentID.HasCertificate() {
		conn.Close()
		return nil, identity.Certificate{}, "", errors.New("local agent identity is not enrolled")
	}
	var assignedUUID string
	linkKey, peerCert, err := share.SoReuseAgentRelayAuthAndExchange(conn, reusePort, agentID, relayChildEnrollment(mgr, childIP, &assignedUUID))
	if err != nil {
		return nil, identity.Certificate{}, "", err
	}
	return linkKey, peerCert, assignedUUID, nil
}

func requestChildUUID(mgr *manager.Manager, childIP string, cert identity.Certificate, wantsEnrollment bool, edPub, xPub []byte) (*protocol.ChildUUIDRes, error) {
	sUMessage := protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)
	header := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.CHILDUUIDREQ,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}
	req := &protocol.ChildUUIDReq{
		ParentUUIDLen:    uint16(len(global.G_Component.UUID)),
		ParentUUID:       global.G_Component.UUID,
		IPLen:            uint16(len(childIP)),
		IP:               childIP,
		WantsEnrollment:  boolToU16(wantsEnrollment),
		Ed25519PublicLen: uint16(len(edPub)),
		Ed25519Public:    edPub,
		X25519PublicLen:  uint16(len(xPub)),
		X25519Public:     xPub,
		Cert:             cert,
	}
	protocol.ConstructMessage(sUMessage, header, req, false)
	sUMessage.SendMessage()
	var res *protocol.ChildUUIDRes
	select {
	case res = <-mgr.ListenManager.ChildUUIDChan:
	case <-time.After(30 * time.Second):
		return nil, errors.New("timeout waiting for admin child identity response")
	}
	if res == nil {
		return nil, errors.New("admin returned empty child identity response")
	}
	if res.OK == 0 {
		if res.Error != "" {
			return nil, errors.New(res.Error)
		}
		return nil, errors.New("admin rejected child identity request")
	}
	return res, nil
}

func relayChildEnrollment(mgr *manager.Manager, childIP string, assignedUUID *string) func(edPub, xPub []byte) (identity.EnrollmentResponse, error) {
	return func(edPub, xPub []byte) (identity.EnrollmentResponse, error) {
		resolvedIP := childIP
		if resolvedIP == "" {
			resolvedIP = "unknown"
		}
		res, err := requestChildUUID(mgr, resolvedIP, identity.Certificate{}, true, edPub, xPub)
		if err != nil {
			return identity.EnrollmentResponse{}, err
		}
		*assignedUUID = res.UUID
		return res.EnrollmentResponse, nil
	}
}

func boolToU16(v bool) uint16 {
	if v {
		return 1
	}
	return 0
}
