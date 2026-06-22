package share

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"Shroud/crypto"
	"Shroud/identity"
)

var AuthKey []byte

var magic = mustRandomMagic()

var ErrPeerNoCert = errors.New("peer does not support certificate authentication")

const (
	authModeCert             byte = 3
	authModeTokenEnroll      byte = 4
	authModeTokenIssueEnroll byte = 5
)

type EnrollInit struct {
	Ed25519Public []byte `json:"ed25519_public"`
	X25519Public  []byte `json:"x25519_public"`
	ClientNonce   []byte `json:"client_nonce,omitempty"`
	ServerNonce   []byte `json:"server_nonce,omitempty"`
	TokenProof    []byte `json:"token_proof,omitempty"`
}

type CertAuthInit struct {
	Cert      identity.Certificate `json:"cert"`
	Signature []byte               `json:"signature"`
}

type CertAuthResponse struct {
	Cert      identity.Certificate `json:"cert"`
	Signature []byte               `json:"signature"`
}

type passiveAuthResult struct {
	mode        byte
	serverNonce []byte
	clientNonce []byte
	peerCert    identity.Certificate
	enrollInit  EnrollInit
}

func GeneratePreAuthToken(secret []byte) {
	AuthKey = crypto.DeriveKey(secret, crypto.PurposeAuth)
}

func SetMagic(newMagic []byte) error {
	if len(newMagic) != 4 {
		return errors.New("preauth magic must be exactly 4 bytes")
	}
	magic = append([]byte(nil), newMagic...)
	return nil
}

func Magic() []byte {
	return append([]byte(nil), magic...)
}

func mustRandomMagic() []byte {
	m := make([]byte, 4)
	if _, err := io.ReadFull(rand.Reader, m); err != nil {
		return []byte{0x53, 0x48, 0x52, 0x44}
	}
	return m
}

func FingerprintFromAuthKey(authKey []byte) (magicBytes []byte, wsPath string) {
	if len(authKey) == 0 {
		return nil, ""
	}
	m := computeHMAC(authKey, []byte("shroud-magic-v1"))[:4]
	p := computeHMAC(authKey, []byte("shroud-ws-path-v1"))[:8]
	return append([]byte(nil), m...), "/" + hex.EncodeToString(p)
}

func ClearPreAuthToken() {
	if AuthKey != nil {
		for i := range AuthKey {
			AuthKey[i] = 0
		}
	}
	AuthKey = nil
}

func computeHMAC(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func readFull(conn net.Conn, buf []byte) error {
	_, err := io.ReadFull(conn, buf)
	return err
}

func writeFull(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func genNonce() ([]byte, error) {
	nonce := make([]byte, 32)
	_, err := io.ReadFull(rand.Reader, nonce)
	return nonce, err
}

func activeTokenAuth(conn net.Conn, mode byte, init *EnrollInit, readServerProof bool) (clientNonce, serverNonce []byte, err error) {
	defer conn.SetReadDeadline(time.Time{})
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if len(AuthKey) == 0 {
		conn.Close()
		return nil, nil, errors.New("missing enrollment key")
	}
	if err = writeFull(conn, magic); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("send magic: %w", err)
	}
	serverNonce = make([]byte, 32)
	if err = readFull(conn, serverNonce); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("read server nonce: %w", err)
	}
	clientNonce, err = genNonce()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	if init == nil {
		init = &EnrollInit{}
	}
	init.ClientNonce = append([]byte(nil), clientNonce...)
	init.ServerNonce = append([]byte(nil), serverNonce...)
	init.TokenProof = enrollmentTokenProof(mode, clientNonce, serverNonce, init.Ed25519Public, init.X25519Public)
	if err = writeFull(conn, []byte{mode}); err != nil {
		conn.Close()
		return nil, nil, err
	}
	if err = writeJSONRecord(conn, init); err != nil {
		conn.Close()
		return nil, nil, err
	}
	if readServerProof {
		serverProof := make([]byte, 32)
		if err = readFull(conn, serverProof); err != nil {
			conn.Close()
			return nil, nil, err
		}
		if subtle.ConstantTimeCompare(serverProof, enrollmentServerProof(mode, clientNonce, serverNonce)) != 1 {
			conn.Close()
			return nil, nil, errors.New("server enrollment authentication failed")
		}
	}
	return clientNonce, serverNonce, nil
}

// ActiveEnrollAndExchange bootstraps a first connection with the one-time enrollment token.
func ActiveEnrollAndExchange(conn net.Conn, agent *identity.AgentStore) ([]byte, error) {
	edPub, xPub, err := agent.PublicKeys()
	if err != nil {
		conn.Close()
		return nil, err
	}
	init := EnrollInit{Ed25519Public: edPub, X25519Public: xPub}
	clientNonce, serverNonce, err := activeTokenAuth(conn, authModeTokenEnroll, &init, true)
	if err != nil {
		return nil, err
	}
	var resp identity.EnrollmentResponse
	if err := readJSONRecord(conn, &resp, 1<<20); err != nil {
		conn.Close()
		return nil, err
	}
	if err := agent.ApplyEnrollment(resp); err != nil {
		conn.Close()
		return nil, err
	}
	return crypto.ECDHExchangeActive(conn, ecdhSalt(clientNonce, serverNonce))
}

// ActiveEnrollAndExchangeWithTokenProof bootstraps first contact without the
// legacy HMAC challenge packet. The token is used only to authenticate the CSR,
// then the issued certificate is persisted and future links use cert auth.
func ActiveEnrollAndExchangeWithTokenProof(conn net.Conn, agent *identity.AgentStore) ([]byte, error) {
	return ActiveEnrollAndExchange(conn, agent)
}

func ActiveAgentAuthAndExchange(conn net.Conn, agent *identity.AgentStore) ([]byte, error) {
	if agent.HasCertificate() {
		return ActiveCertAuthAndExchange(conn, agent)
	}
	return ActiveEnrollAndExchange(conn, agent)
}

func PassiveAdminAuthAndExchange(conn net.Conn, admin *identity.AdminStore) ([]byte, identity.Certificate, error) {
	res, err := passiveAuthDispatchFull(conn, &certPassiveConfig{verifier: admin, localCert: admin.NodeCert, localPriv: admin.NodePrivateKey()})
	if err != nil {
		return nil, identity.Certificate{}, err
	}
	switch res.mode {
	case authModeCert:
		linkKey, err := crypto.ECDHExchangePassive(conn, ecdhSalt(res.clientNonce, res.serverNonce))
		return linkKey, res.peerCert, err
	case authModeTokenEnroll:
		resp, err := admin.EnrollAgent(AuthKey, res.enrollInit.Ed25519Public, res.enrollInit.X25519Public)
		if err != nil {
			conn.Close()
			return nil, identity.Certificate{}, err
		}
		if err := writeJSONRecord(conn, resp); err != nil {
			conn.Close()
			return nil, identity.Certificate{}, err
		}
		linkKey, err := crypto.ECDHExchangePassive(conn, ecdhSalt(res.clientNonce, res.serverNonce))
		return linkKey, resp.AgentCert, err
	default:
		conn.Close()
		return nil, identity.Certificate{}, errors.New("unexpected auth mode for admin passive")
	}
}

// PassiveAdminAuthAndExchangeNegotiated is the single passive admin entrypoint:
// it reads the mode byte once and dispatches to certificate auth or token-based
// enrollment without retrying on a consumed connection.
func PassiveAdminAuthAndExchangeNegotiated(conn net.Conn, admin *identity.AdminStore) ([]byte, identity.Certificate, error) {
	return PassiveAdminAuthAndExchange(conn, admin)
}

func PassiveAgentAuthAndExchange(conn net.Conn, agent *identity.AgentStore) ([]byte, identity.Certificate, error) {
	if agent.HasCertificate() {
		return PassiveCertAuthAndExchange(conn, agent, agent.Cert, agent.NodePrivateKey())
	}
	linkKey, err := PassiveRequestEnrollAndExchange(conn, agent)
	return linkKey, identity.Certificate{}, err
}

func ActiveAdminAuthAndExchange(conn net.Conn, admin *identity.AdminStore) ([]byte, identity.Certificate, error) {
	return ActiveAdminCertAuthAndExchange(conn, admin, admin)
}

func ActiveAdminIssueEnrollAndExchange(conn net.Conn, admin *identity.AdminStore) ([]byte, identity.Certificate, error) {
	return ActiveIssueEnrollAndExchange(conn, admin)
}

// PassiveEnrollAndExchange accepts a token-authenticated first connection and issues a node cert.
func PassiveEnrollAndExchange(conn net.Conn, admin *identity.AdminStore) ([]byte, identity.Certificate, error) {
	res, err := passiveAuthDispatchFull(conn, nil)
	if err != nil {
		return nil, identity.Certificate{}, err
	}
	if res.mode != authModeTokenEnroll {
		conn.Close()
		return nil, identity.Certificate{}, errors.New("unexpected non-enrollment auth mode")
	}
	init := res.enrollInit
	resp, err := admin.EnrollAgent(AuthKey, init.Ed25519Public, init.X25519Public)
	if err != nil {
		conn.Close()
		return nil, identity.Certificate{}, err
	}
	if err := writeJSONRecord(conn, resp); err != nil {
		conn.Close()
		return nil, identity.Certificate{}, err
	}
	linkKey, err := crypto.ECDHExchangePassive(conn, ecdhSalt(res.clientNonce, res.serverNonce))
	return linkKey, resp.AgentCert, err
}

// ActiveIssueEnrollAndExchange is used when admin actively connects to a never-enrolled passive agent.
func ActiveIssueEnrollAndExchange(conn net.Conn, admin *identity.AdminStore) ([]byte, identity.Certificate, error) {
	clientNonce, serverNonce, err := activeTokenAuth(conn, authModeTokenIssueEnroll, &EnrollInit{}, true)
	if err != nil {
		return nil, identity.Certificate{}, err
	}
	var init EnrollInit
	if err := readJSONRecord(conn, &init, 1<<20); err != nil {
		conn.Close()
		return nil, identity.Certificate{}, err
	}
	resp, err := admin.EnrollAgent(AuthKey, init.Ed25519Public, init.X25519Public)
	if err != nil {
		conn.Close()
		return nil, identity.Certificate{}, err
	}
	if err := writeJSONRecord(conn, resp); err != nil {
		conn.Close()
		return nil, identity.Certificate{}, err
	}
	linkKey, err := crypto.ECDHExchangeActive(conn, ecdhSalt(clientNonce, serverNonce))
	return linkKey, resp.AgentCert, err
}

func ActiveAgentRelayEnrollAndExchange(conn net.Conn, agent *identity.AgentStore, relay func(edPub, xPub []byte) (identity.EnrollmentResponse, error)) ([]byte, identity.Certificate, error) {
	clientNonce, serverNonce, err := activeTokenAuth(conn, authModeTokenIssueEnroll, &EnrollInit{}, true)
	if err != nil {
		return nil, identity.Certificate{}, err
	}
	var init EnrollInit
	if err := readJSONRecord(conn, &init, 1<<20); err != nil {
		conn.Close()
		return nil, identity.Certificate{}, err
	}
	resp, err := relay(init.Ed25519Public, init.X25519Public)
	if err != nil {
		conn.Close()
		return nil, identity.Certificate{}, err
	}
	if err := writeJSONRecord(conn, resp); err != nil {
		conn.Close()
		return nil, identity.Certificate{}, err
	}
	linkKey, err := crypto.ECDHExchangeActive(conn, ecdhSalt(clientNonce, serverNonce))
	return linkKey, resp.AgentCert, err
}

func PassiveAgentEnrollRelayOrCertAndExchange(conn net.Conn, agent *identity.AgentStore, relay func(edPub, xPub []byte) (identity.EnrollmentResponse, error)) ([]byte, identity.Certificate, error) {
	res, err := passiveAuthDispatchFull(conn, &certPassiveConfig{verifier: agent, localCert: agent.Cert, localPriv: agent.NodePrivateKey()})
	if err != nil {
		return nil, identity.Certificate{}, err
	}
	switch res.mode {
	case authModeCert:
		linkKey, err := crypto.ECDHExchangePassive(conn, ecdhSalt(res.clientNonce, res.serverNonce))
		return linkKey, res.peerCert, err
	case authModeTokenEnroll:
		resp, err := relay(res.enrollInit.Ed25519Public, res.enrollInit.X25519Public)
		if err != nil {
			conn.Close()
			return nil, identity.Certificate{}, err
		}
		if err := writeJSONRecord(conn, resp); err != nil {
			conn.Close()
			return nil, identity.Certificate{}, err
		}
		linkKey, err := crypto.ECDHExchangePassive(conn, ecdhSalt(res.clientNonce, res.serverNonce))
		return linkKey, resp.AgentCert, err
	default:
		conn.Close()
		return nil, identity.Certificate{}, errors.New("unexpected auth mode for agent relay")
	}
}

// PassiveRequestEnrollAndExchange is used when an unenrolled passive agent is contacted by admin.
func PassiveRequestEnrollAndExchange(conn net.Conn, agent *identity.AgentStore) ([]byte, error) {
	res, err := passiveAuthDispatchFull(conn, nil)
	if err != nil {
		return nil, err
	}
	if res.mode != authModeTokenIssueEnroll {
		conn.Close()
		return nil, errors.New("unexpected non-enrollment auth mode")
	}
	return PassiveRequestEnrollAfterChallenge(conn, agent, res.clientNonce, res.serverNonce)
}

func PassiveRequestEnrollAfterChallenge(conn net.Conn, agent *identity.AgentStore, clientNonce, serverNonce []byte) ([]byte, error) {
	edPub, xPub, err := agent.PublicKeys()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := writeJSONRecord(conn, EnrollInit{Ed25519Public: edPub, X25519Public: xPub}); err != nil {
		conn.Close()
		return nil, err
	}
	var resp identity.EnrollmentResponse
	if err := readJSONRecord(conn, &resp, 1<<20); err != nil {
		conn.Close()
		return nil, err
	}
	if err := agent.ApplyEnrollment(resp); err != nil {
		conn.Close()
		return nil, err
	}
	return crypto.ECDHExchangePassive(conn, ecdhSalt(clientNonce, serverNonce))
}

func ActiveCertAuthAndExchange(conn net.Conn, agent *identity.AgentStore) ([]byte, error) {
	clientNonce, serverNonce, err := activeCertAuth(conn, agent)
	if err != nil {
		return nil, err
	}
	return crypto.ECDHExchangeActive(conn, ecdhSalt(clientNonce, serverNonce))
}

func ActiveAgentCertAuthAndExchange(conn net.Conn, agent *identity.AgentStore) ([]byte, identity.Certificate, error) {
	clientNonce, serverNonce, peerCert, err := activeCertAuthWithMaterial(conn, agent.Cert, agent.NodePrivateKey(), agent)
	if err != nil {
		return nil, identity.Certificate{}, err
	}
	linkKey, err := crypto.ECDHExchangeActive(conn, ecdhSalt(clientNonce, serverNonce))
	return linkKey, peerCert, err
}

func PassiveCertAuthAndExchange(conn net.Conn, verifier interface {
	VerifyPeerCertificate(identity.Certificate) error
}, localCert identity.Certificate, localPriv ed25519.PrivateKey) ([]byte, identity.Certificate, error) {
	mode, serverNonce, clientNonce, peerCert, err := passiveAuthDispatch(conn, &certPassiveConfig{verifier: verifier, localCert: localCert, localPriv: localPriv})
	if err != nil {
		return nil, identity.Certificate{}, err
	}
	if mode != authModeCert {
		conn.Close()
		return nil, identity.Certificate{}, errors.New("unexpected non-certificate auth mode")
	}
	linkKey, err := crypto.ECDHExchangePassive(conn, ecdhSalt(clientNonce, serverNonce))
	return linkKey, peerCert, err
}

func ActiveAdminCertAuthAndExchange(conn net.Conn, admin *identity.AdminStore, verifier interface {
	VerifyPeerCertificate(identity.Certificate) error
}) ([]byte, identity.Certificate, error) {
	clientNonce, serverNonce, peerCert, err := activeCertAuthWithMaterial(conn, admin.NodeCert, admin.NodePrivateKey(), verifier)
	if err != nil {
		return nil, identity.Certificate{}, err
	}
	linkKey, err := crypto.ECDHExchangeActive(conn, ecdhSalt(clientNonce, serverNonce))
	return linkKey, peerCert, err
}

func SoReuseAgentAuthAndExchange(conn net.Conn, reusePort string, agent *identity.AgentStore) ([]byte, identity.Certificate, error) {
	res, err := soReuseDispatchFull(conn, reusePort, &certPassiveConfig{verifier: agent, localCert: agent.Cert, localPriv: agent.NodePrivateKey()})
	if err != nil {
		return nil, identity.Certificate{}, err
	}
	switch res.mode {
	case authModeCert:
		linkKey, err := crypto.ECDHExchangePassive(conn, ecdhSalt(res.clientNonce, res.serverNonce))
		return linkKey, res.peerCert, err
	case authModeTokenIssueEnroll:
		linkKey, err := PassiveRequestEnrollAfterChallenge(conn, agent, res.clientNonce, res.serverNonce)
		return linkKey, identity.Certificate{}, err
	default:
		conn.Close()
		return nil, identity.Certificate{}, errors.New("unexpected auth mode")
	}
}

func SoReuseAgentRelayAuthAndExchange(conn net.Conn, reusePort string, agent *identity.AgentStore, relay func(edPub, xPub []byte) (identity.EnrollmentResponse, error)) ([]byte, identity.Certificate, error) {
	res, err := soReuseDispatchFull(conn, reusePort, &certPassiveConfig{verifier: agent, localCert: agent.Cert, localPriv: agent.NodePrivateKey()})
	if err != nil {
		return nil, identity.Certificate{}, err
	}
	switch res.mode {
	case authModeCert:
		linkKey, err := crypto.ECDHExchangePassive(conn, ecdhSalt(res.clientNonce, res.serverNonce))
		return linkKey, res.peerCert, err
	case authModeTokenEnroll:
		init := res.enrollInit
		resp, err := relay(init.Ed25519Public, init.X25519Public)
		if err != nil {
			conn.Close()
			return nil, identity.Certificate{}, err
		}
		if err := writeJSONRecord(conn, resp); err != nil {
			conn.Close()
			return nil, identity.Certificate{}, err
		}
		linkKey, err := crypto.ECDHExchangePassive(conn, ecdhSalt(res.clientNonce, res.serverNonce))
		return linkKey, resp.AgentCert, err
	default:
		conn.Close()
		return nil, identity.Certificate{}, errors.New("unexpected auth mode")
	}
}

type certPassiveConfig struct {
	verifier interface {
		VerifyPeerCertificate(identity.Certificate) error
	}
	localCert identity.Certificate
	localPriv ed25519.PrivateKey
}

func activeCertAuth(conn net.Conn, agent *identity.AgentStore) (clientNonce, serverNonce []byte, err error) {
	clientNonce, serverNonce, _, err = activeCertAuthWithMaterial(conn, agent.Cert, agent.NodePrivateKey(), agent)
	return clientNonce, serverNonce, err
}

func activeCertAuthWithMaterial(conn net.Conn, localCert identity.Certificate, localPriv ed25519.PrivateKey, verifier interface {
	VerifyPeerCertificate(identity.Certificate) error
}) (clientNonce, serverNonce []byte, peerCert identity.Certificate, err error) {
	if conn == nil {
		return nil, nil, peerCert, errors.New("nil connection")
	}
	defer conn.SetReadDeadline(time.Time{})
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if len(localCert.Signature) == 0 {
		conn.Close()
		return nil, nil, peerCert, identity.ErrNoCertificate
	}
	if err = writeFull(conn, magic); err != nil {
		conn.Close()
		return nil, nil, peerCert, fmt.Errorf("send magic: %w", err)
	}
	serverNonce = make([]byte, 32)
	if err = readFull(conn, serverNonce); err != nil {
		conn.Close()
		return nil, nil, peerCert, fmt.Errorf("read server nonce: %w", err)
	}
	clientNonce, err = genNonce()
	if err != nil {
		conn.Close()
		return nil, nil, peerCert, err
	}
	sig := ed25519.Sign(localPriv, certAuthTranscript(authModeCert, clientNonce, serverNonce, localCert.Serial))
	if err = writeFull(conn, []byte{authModeCert}); err != nil {
		conn.Close()
		return nil, nil, peerCert, err
	}
	if err = writeJSONRecord(conn, struct {
		ClientNonce []byte       `json:"client_nonce"`
		Init        CertAuthInit `json:"init"`
	}{ClientNonce: clientNonce, Init: CertAuthInit{Cert: localCert, Signature: sig}}); err != nil {
		conn.Close()
		return nil, nil, peerCert, err
	}
	var resp CertAuthResponse
	if err = readJSONRecord(conn, &resp, 1<<20); err != nil {
		conn.Close()
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, nil, peerCert, fmt.Errorf("%w: %v", ErrPeerNoCert, err)
		}
		return nil, nil, peerCert, err
	}
	if verifier != nil {
		if err = verifier.VerifyPeerCertificate(resp.Cert); err != nil {
			conn.Close()
			return nil, nil, peerCert, err
		}
	}
	if !ed25519.Verify(ed25519.PublicKey(resp.Cert.Ed25519Public), certAuthTranscript(authModeCert, serverNonce, clientNonce, resp.Cert.Serial), resp.Signature) {
		conn.Close()
		return nil, nil, peerCert, errors.New("invalid server certificate-auth signature")
	}
	return clientNonce, serverNonce, resp.Cert, nil
}

func passiveAuthDispatch(conn net.Conn, cfg *certPassiveConfig) (mode byte, serverNonce, clientNonce []byte, peerCert identity.Certificate, err error) {
	res, err := passiveAuthDispatchFull(conn, cfg)
	if err != nil {
		return 0, nil, nil, peerCert, err
	}
	return res.mode, res.serverNonce, res.clientNonce, res.peerCert, nil
}

func passiveAuthDispatchFull(conn net.Conn, cfg *certPassiveConfig) (passiveAuthResult, error) {
	var res passiveAuthResult
	defer conn.SetReadDeadline(time.Time{})
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	magicBuf := make([]byte, 4)
	if err := readFull(conn, magicBuf); err != nil {
		conn.Close()
		return res, fmt.Errorf("read magic: %w", err)
	}
	if string(magicBuf) != string(magic) {
		conn.Close()
		return res, errors.New("invalid magic")
	}
	return passiveAuthDispatchAfterMagicFull(conn, cfg)
}

func passiveAuthDispatchAfterMagic(conn net.Conn, cfg *certPassiveConfig) (mode byte, serverNonce, clientNonce []byte, peerCert identity.Certificate, err error) {
	res, err := passiveAuthDispatchAfterMagicFull(conn, cfg)
	if err != nil {
		return 0, nil, nil, peerCert, err
	}
	return res.mode, res.serverNonce, res.clientNonce, res.peerCert, nil
}

func passiveAuthDispatchAfterMagicFull(conn net.Conn, cfg *certPassiveConfig) (passiveAuthResult, error) {
	var res passiveAuthResult
	var err error
	res.serverNonce, err = genNonce()
	if err != nil {
		conn.Close()
		return res, fmt.Errorf("gen nonce: %w", err)
	}
	if err = writeFull(conn, res.serverNonce); err != nil {
		conn.Close()
		return res, fmt.Errorf("send nonce: %w", err)
	}

	first := make([]byte, 1)
	if err = readFull(conn, first); err != nil {
		conn.Close()
		return res, err
	}
	res.mode = first[0]
	switch res.mode {
	case authModeTokenEnroll, authModeTokenIssueEnroll:
		if err = readJSONRecord(conn, &res.enrollInit, 1<<20); err != nil {
			conn.Close()
			return res, err
		}
		if err := validateEnrollmentInit(res.mode, res.serverNonce, res.enrollInit); err != nil {
			conn.Close()
			return res, err
		}
		res.clientNonce = res.enrollInit.ClientNonce
		if err := writeFull(conn, enrollmentServerProof(res.mode, res.clientNonce, res.serverNonce)); err != nil {
			conn.Close()
			return res, err
		}
		return res, nil
	case authModeCert:
		var req struct {
			ClientNonce []byte       `json:"client_nonce"`
			Init        CertAuthInit `json:"init"`
		}
		if err = readJSONRecord(conn, &req, 1<<20); err != nil {
			conn.Close()
			return res, err
		}
		res.clientNonce = req.ClientNonce
		res.peerCert = req.Init.Cert
		if cfg == nil || cfg.verifier == nil {
			conn.Close()
			return res, errors.New("certificate auth not configured")
		}
		if err = cfg.verifier.VerifyPeerCertificate(res.peerCert); err != nil {
			conn.Close()
			return res, err
		}
		if !ed25519.Verify(ed25519.PublicKey(res.peerCert.Ed25519Public), certAuthTranscript(authModeCert, res.clientNonce, res.serverNonce, res.peerCert.Serial), req.Init.Signature) {
			conn.Close()
			return res, errors.New("invalid client certificate-auth signature")
		}
		sig := ed25519.Sign(cfg.localPriv, certAuthTranscript(authModeCert, res.serverNonce, res.clientNonce, cfg.localCert.Serial))
		if err = writeJSONRecord(conn, CertAuthResponse{Cert: cfg.localCert, Signature: sig}); err != nil {
			conn.Close()
			return res, err
		}
		return res, nil
	default:
		conn.Close()
		return res, fmt.Errorf("unknown auth mode %d", res.mode)
	}
}

func soReuseDispatch(conn net.Conn, reusePort string, cfg *certPassiveConfig) (mode byte, serverNonce, clientNonce []byte, peerCert identity.Certificate, err error) {
	res, err := soReuseDispatchFull(conn, reusePort, cfg)
	if err != nil {
		return 0, nil, nil, peerCert, err
	}
	return res.mode, res.serverNonce, res.clientNonce, res.peerCert, nil
}

func soReuseDispatchFull(conn net.Conn, reusePort string, cfg *certPassiveConfig) (passiveAuthResult, error) {
	var res passiveAuthResult
	defer conn.SetReadDeadline(time.Time{})
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	magicBuf := make([]byte, 4)
	count, err := io.ReadFull(conn, magicBuf)
	if err != nil {
		if timeoutErr, ok := err.(net.Error); ok && timeoutErr.Timeout() {
			go proxyStreamFunc(conn, magicBuf[:count], reusePort)
			return res, fmt.Errorf("timeout: non-shroud connection")
		}
		conn.Close()
		return res, err
	}
	if string(magicBuf) != string(magic) {
		go proxyStreamFunc(conn, magicBuf[:count], reusePort)
		return res, fmt.Errorf("non-shroud connection")
	}
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	return passiveAuthDispatchAfterMagicFull(conn, cfg)
}

func ecdhSalt(clientNonce, serverNonce []byte) []byte {
	return append(append([]byte(nil), clientNonce...), serverNonce...)
}

func certAuthTranscript(mode byte, firstNonce, secondNonce []byte, serial string) []byte {
	b := make([]byte, 0, 1+32+32+len(serial)+32)
	b = append(b, []byte("shroud-cert-auth-v1")...)
	b = append(b, mode)
	b = append(b, firstNonce...)
	b = append(b, secondNonce...)
	b = append(b, []byte(serial)...)
	return b
}

func enrollmentTokenProof(mode byte, clientNonce, serverNonce, edPub, xPub []byte) []byte {
	mac := hmac.New(sha256.New, AuthKey)
	mac.Write([]byte("shroud-enroll-token-v1"))
	mac.Write([]byte{mode})
	mac.Write(clientNonce)
	mac.Write(serverNonce)
	mac.Write(edPub)
	mac.Write(xPub)
	return mac.Sum(nil)
}

func enrollmentServerProof(mode byte, clientNonce, serverNonce []byte) []byte {
	mac := hmac.New(sha256.New, AuthKey)
	mac.Write([]byte("shroud-enroll-server-v1"))
	mac.Write([]byte{mode})
	mac.Write(clientNonce)
	mac.Write(serverNonce)
	return mac.Sum(nil)
}

func validateEnrollmentInit(mode byte, serverNonce []byte, init EnrollInit) error {
	if len(AuthKey) == 0 {
		return errors.New("missing enrollment key")
	}
	if len(init.ClientNonce) != 32 || len(init.ServerNonce) != 32 {
		return errors.New("invalid enrollment nonce")
	}
	if subtle.ConstantTimeCompare(init.ServerNonce, serverNonce) != 1 {
		return errors.New("enrollment server nonce mismatch")
	}
	expected := enrollmentTokenProof(mode, init.ClientNonce, serverNonce, init.Ed25519Public, init.X25519Public)
	if subtle.ConstantTimeCompare(init.TokenProof, expected) != 1 {
		return errors.New("invalid enrollment token proof")
	}
	return nil
}

func writeJSONRecord(conn net.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if err := writeFull(conn, lenBuf[:]); err != nil {
		return err
	}
	return writeFull(conn, data)
}

func readJSONRecord(conn net.Conn, v any, max uint32) error {
	var lenBuf [4]byte
	if err := readFull(conn, lenBuf[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(lenBuf[:])
	if n > max {
		return fmt.Errorf("json record too large: %d", n)
	}
	data := make([]byte, n)
	if err := readFull(conn, data); err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

var proxyStreamFunc func(conn net.Conn, data []byte, port string) = func(conn net.Conn, data []byte, port string) { conn.Close() }

func SetProxyStreamFunc(f func(net.Conn, []byte, string)) { proxyStreamFunc = f }
