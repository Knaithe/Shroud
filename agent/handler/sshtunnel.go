package handler

import (
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"Shroud/agent/manager"
	"Shroud/global"
	"Shroud/identity"
	"Shroud/protocol"
	"Shroud/share"

	"golang.org/x/crypto/ssh"
)

type SSHTunnel struct {
	Method             int
	Addr               string
	Port               string
	Username           string
	Password           string
	Certificate        []byte
	HostKeyFingerprint string
}

func newSSHTunnel(method int, addr, port, username, password string, certificate []byte, hostKeyFingerprint string) *SSHTunnel {
	sshTunnel := new(SSHTunnel)
	sshTunnel.Method = method
	sshTunnel.Addr = addr
	sshTunnel.Port = port
	sshTunnel.Username = username
	sshTunnel.Password = password
	sshTunnel.Certificate = certificate
	sshTunnel.HostKeyFingerprint = hostKeyFingerprint
	return sshTunnel
}

func (sshTunnel *SSHTunnel) start(mgr *manager.Manager) {
	var authPayload ssh.AuthMethod
	var err error
	var sUMessage, sLMessage, rMessage protocol.Message

	sUMessage = protocol.NewUpMsg(global.G_Component.Conn, global.G_Component.CryptoKey, global.Session.GetLinkKey(), global.G_Component.UUID)

	sshTunnelResheader := &protocol.Header{
		Sender:      global.G_Component.UUID,
		Accepter:    protocol.ADMIN_UUID,
		MessageType: protocol.SSHTUNNELRES,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))), // No need to set route when agent send mess to admin
		Route:       protocol.TEMP_ROUTE,
	}

	sshTunnelResSuccMess := &protocol.SSHTunnelRes{
		OK: 1,
	}

	sshTunnelResFailMess := &protocol.SSHTunnelRes{
		OK: 0,
	}

	defer func() {
		if err != nil {
			protocol.ConstructMessage(sUMessage, sshTunnelResheader, sshTunnelResFailMess, false)
			sUMessage.SendMessage()
		}
	}()

	switch sshTunnel.Method {
	case UPMETHOD:
		authPayload = ssh.Password(sshTunnel.Password)
	case CERMETHOD:
		var key ssh.Signer
		key, err = ssh.ParsePrivateKey(sshTunnel.Certificate)
		if err != nil {
			return
		}
		authPayload = ssh.PublicKeys(key)
	}

	sshDial, err := ssh.Dial("tcp", sshTunnel.Addr, &ssh.ClientConfig{
		User: sshTunnel.Username,
		Auth: []ssh.AuthMethod{authPayload},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fp := ssh.FingerprintSHA256(key)
			if sshTunnel.HostKeyFingerprint != "" {
				if fp != sshTunnel.HostKeyFingerprint {
					return fmt.Errorf("SSH host key mismatch: got %s, expected %s", fp, sshTunnel.HostKeyFingerprint)
				}
				log.Printf("[*] SSH host key verified for %s (%s): %s", hostname, remote, fp)
			} else {
				log.Printf("[*] WARNING: SSH TOFU mode. Host key for %s (%s): %s", hostname, remote, fp)
			}
			return nil
		},
		Timeout: 10 * time.Second,
	})
	if err != nil {
		return
	}

	var linkKey []byte
	var conn net.Conn
	var peerCert identity.Certificate
	var preassignedUUID string
	dialChild := func() (net.Conn, error) {
		return sshDial.Dial("tcp", fmt.Sprintf("127.0.0.1:%s", sshTunnel.Port))
	}
	conn, linkKey, peerCert, preassignedUUID, err = activeChildAuthWithRetry(dialChild, mgr)
	if err != nil {
		return
	}

	sLMessage = protocol.NewDownMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.ADMIN_UUID)

	hiHeader := &protocol.Header{
		Sender:      protocol.ADMIN_UUID, // fake admin
		Accepter:    protocol.TEMP_UUID,
		MessageType: protocol.HI,
		RouteLen:    uint32(len([]byte(protocol.TEMP_ROUTE))),
		Route:       protocol.TEMP_ROUTE,
	}

	// fake admin
	hiMess := &protocol.HIMess{
		GreetingLen: uint16(len(share.GreetHello())),
		Greeting:    share.GreetHello(),
		UUIDLen:     uint16(len(protocol.ADMIN_UUID)),
		UUID:        protocol.ADMIN_UUID,
		IsAdmin:     1,
		IsReconnect: 0,
	}

	protocol.ConstructMessage(sLMessage, hiHeader, hiMess, false)
	sLMessage.SendMessage()

	rMessage = protocol.NewDownMsg(conn, global.G_Component.CryptoKey, linkKey, protocol.ADMIN_UUID)
	fHeader, fMessage, err := protocol.DestructMessage(rMessage)
	if err != nil {
		conn.Close()
		return
	}

	if fHeader.MessageType == protocol.HI {
		mmess := fMessage.(*protocol.HIMess)
		if mmess.Greeting == share.GreetAck() && mmess.IsAdmin == 0 {
			childIP := conn.RemoteAddr().String()

			var childUUID string
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
				Sender:      protocol.ADMIN_UUID,
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

			mgr.ChildrenManager.NewChild(childUUID, conn, linkKey)

			mgr.ChildrenManager.ChildComeChan <- &manager.ChildInfo{UUID: childUUID, Conn: conn}

			protocol.ConstructMessage(sUMessage, sshTunnelResheader, sshTunnelResSuccMess, false)
			sUMessage.SendMessage()

			return
		}
	}

	conn.Close()
	err = errors.New("node looks invalid")
}

func DispatchSSHTunnelMess(mgr *manager.Manager) {
	for {
		message := <-mgr.SSHTunnelManager.SSHTunnelMessChan

		switch mess := message.(type) {
		case *protocol.SSHTunnelReq:
			sshTunnel := newSSHTunnel(int(mess.Method), mess.Addr, mess.Port, mess.Username, mess.Password, mess.Certificate, mess.HostKeyFingerprint)
			go sshTunnel.start(mgr)
		}
	}
}
