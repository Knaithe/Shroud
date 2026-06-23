package global

import (
	"net"
	"os"
	"sync"

	"Shroud/identity"
	"Shroud/protocol"
	"Shroud/utils"
)

var Session *SessionConfig

type SessionConfig struct {
	mu sync.Mutex

	Component       *protocol.MessageComponent
	linkKey         []byte
	TLSEnable       bool
	TLSFingerprint  string
	TLSInsecure     bool
	TorProxy        string
	transportMode   string
	TransportSwitch chan struct{}
	AdminIdentity   *identity.AdminStore
	AgentIdentity   *identity.AgentStore
	PeerCert        identity.Certificate
}

func InitSession(conn net.Conn, cryptoKey []byte, uuid string) {
	Session = &SessionConfig{
		Component: &protocol.MessageComponent{
			CryptoKey: cryptoKey,
			Conn:      conn,
			UUID:      uuid,
		},
		transportMode:   "raw",
		TransportSwitch: make(chan struct{}, 1),
	}
}

func (s *SessionConfig) UpdateConn(conn net.Conn) {
	s.mu.Lock()
	s.Component.Conn = utils.WrapConn(conn)
	s.mu.Unlock()
}

func (s *SessionConfig) SwapConn(newConn net.Conn) net.Conn {
	s.mu.Lock()
	oldConn := s.Component.Conn
	s.Component.Conn = utils.WrapConn(newConn)
	s.mu.Unlock()
	return oldConn
}

func (s *SessionConfig) SignalTransportSwitch() {
	select {
	case s.TransportSwitch <- struct{}{}:
	default:
	}
}

func (s *SessionConfig) SetLinkKey(key []byte) {
	s.mu.Lock()
	s.linkKey = key
	s.mu.Unlock()
}

func (s *SessionConfig) GetLinkKey() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.linkKey
}

func (s *SessionConfig) SetTransportMode(mode string) {
	s.mu.Lock()
	s.transportMode = mode
	s.mu.Unlock()
}

func (s *SessionConfig) GetTransportMode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transportMode
}

// Backward-compatible accessors — callers use global.G_Component unchanged.

var G_Component *protocol.MessageComponent

func InitialGComponent(conn net.Conn, cryptoKey []byte, uuid string) {
	InitSession(conn, cryptoKey, uuid)
	G_Component = Session.Component
}

func UpdateGComponent(conn net.Conn) {
	Session.UpdateConn(conn)
}

func SwapGComponentConn(newConn net.Conn) net.Conn {
	return Session.SwapConn(newConn)
}

func SignalTransportSwitch() {
	Session.SignalTransportSwitch()
}

func SetTransportMode(mode string) {
	Session.SetTransportMode(mode)
}

func GetTransportMode() string {
	return Session.GetTransportMode()
}

// AdminCleanExit is set by admin/admin.go after globals are initialized.
// It wipes cryptographic seeds and keys, closes the connection, then exits.
// Default no-op prevents nil pointer crash if triggered before admin startup completes.
var AdminCleanExit = func() { os.Exit(1) }

// Consolidated globals — previously scattered individual vars.

func init() {
	G_Component = nil // will be set by InitialGComponent
}

func SetAdminIdentity(st *identity.AdminStore) {
	if Session != nil {
		Session.AdminIdentity = st
	}
	if G_Component != nil {
		G_Component.CommandSigner = st
		G_Component.E2EKeyResolver = st.PayloadKeyForPeerUUID
	}
}
func SetAgentIdentity(st *identity.AgentStore) {
	if Session != nil {
		Session.AgentIdentity = st
	}
	if G_Component != nil {
		G_Component.CommandVerifier = st
		G_Component.E2EKeyResolver = func(string) []byte { return st.PayloadKeyForAdmin() }
		G_Component.E2EKey = st.PayloadKeyForAdmin()
	}
}
func SetPeerCert(cert identity.Certificate) {
	if Session != nil {
		Session.PeerCert = cert
	}
}
