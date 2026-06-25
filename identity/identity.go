package identity

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	scrypto "Shroud/crypto"

	"golang.org/x/crypto/hkdf"
)

const (
	CertVersion = 1

	RoleAdmin         = "admin"
	RoleAgent         = "agent"
	RoleCommandSigner = "command-signer"

	UsageNode          = "node-auth"
	UsageCommandSigner = "command-sign"

	certTTL           = 30 * 24 * time.Hour
	commandTTL        = 5 * time.Minute
	e2eInfoPrefix     = "shroud-e2e-v1"
	certSignDomain    = "shroud-cert-v1"
	commandSignDomain = "shroud-command-v1"
)

var (
	ErrNoCertificate = errors.New("identity has no signed certificate")
	ErrRevoked       = errors.New("certificate is revoked")
)

type Certificate struct {
	Version       int      `json:"version"`
	Serial        string   `json:"serial"`
	NodeID        string   `json:"node_id"`
	Role          string   `json:"role"`
	Usages        []string `json:"usages"`
	Ed25519Public []byte   `json:"ed25519_public"`
	X25519Public  []byte   `json:"x25519_public,omitempty"`
	IssuedAt      int64    `json:"issued_at"`
	ExpiresAt     int64    `json:"expires_at"`
	Signature     []byte   `json:"signature"`
}

type AdminStore struct {
	mu   sync.Mutex `json:"-"`
	Path string     `json:"-"`

	CASeed   []byte `json:"ca_seed"`
	CAPublic []byte `json:"ca_public"`

	NodeSeed       []byte      `json:"node_seed"`
	NodeX25519Priv []byte      `json:"node_x25519_private"`
	NodeCert       Certificate `json:"node_cert"`

	CommandSeed []byte      `json:"command_seed"`
	CommandCert Certificate `json:"command_cert"`

	AgentCerts             map[string]Certificate `json:"agent_certs"`             // serial -> cert
	ProtocolUUIDToSerial   map[string]string      `json:"protocol_uuid_to_serial"` // runtime UUID -> cert serial
	RevokedSerials         map[string]int64       `json:"revoked_serials"`
	ConsumedEnrollmentKeys map[string]int64       `json:"consumed_enrollment_keys"`
	NextCommandSeq         uint64                 `json:"next_command_seq"`
	ProtocolMagic          []byte                 `json:"protocol_magic"`
	WebSocketPath          string                 `json:"websocket_path"`
	AdminUUID              string                 `json:"admin_uuid,omitempty"`
}

type AgentStore struct {
	mu   sync.Mutex `json:"-"`
	Path string     `json:"-"`

	NodeSeed       []byte      `json:"node_seed"`
	NodeX25519Priv []byte      `json:"node_x25519_private"`
	Cert           Certificate `json:"cert"`

	CAPublic         []byte            `json:"ca_public"`
	AdminNodeCert    Certificate       `json:"admin_node_cert"`
	AdminCommandCert Certificate       `json:"admin_command_cert"`
	RevokedSerials   map[string]int64  `json:"revoked_serials"`
	LastCommandSeq   map[string]uint64 `json:"last_command_seq"`
	ProtocolMagic    []byte            `json:"protocol_magic"`
	WebSocketPath    string            `json:"websocket_path"`
	AdminUUID        string            `json:"admin_uuid,omitempty"`
}

type CAStore struct {
	CASeed   []byte `json:"ca_seed"`
	CAPublic []byte `json:"ca_public"`
}

func LoadCA(path string) (*CAStore, error) {
	data, err := readFileData(path)
	if err != nil {
		return nil, err
	}
	ca := &CAStore{}
	if err := json.Unmarshal(data, ca); err != nil {
		return nil, err
	}
	return ca, nil
}

func (s *AdminStore) SetExternalCA(ca *CAStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CASeed = append([]byte(nil), ca.CASeed...)
	s.CAPublic = append([]byte(nil), ca.CAPublic...)
}

func (s *AdminStore) ExportCA(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ca := &CAStore{
		CASeed:   append([]byte(nil), s.CASeed...),
		CAPublic: append([]byte(nil), s.CAPublic...),
	}
	return writeJSONFile(path, ca)
}

func (s *AdminStore) ClearCASeedForSave() {
	s.mu.Lock()
	s.CASeed = nil
	s.mu.Unlock()
	_ = s.Save()
}

type EnrollmentResponse struct {
	AgentCert        Certificate      `json:"agent_cert"`
	CAPublic         []byte           `json:"ca_public"`
	AdminNodeCert    Certificate      `json:"admin_node_cert"`
	AdminCommandCert Certificate      `json:"admin_command_cert"`
	RevokedSerials   map[string]int64 `json:"revoked_serials"`
	ProtocolMagic    []byte           `json:"protocol_magic"`
	WebSocketPath    string           `json:"websocket_path"`
	AdminUUID        string           `json:"admin_uuid,omitempty"`
}

type SignedPayload struct {
	Version    int         `json:"version"`
	Sequence   uint64      `json:"sequence"`
	Timestamp  int64       `json:"timestamp"`
	SignerCert Certificate `json:"signer_cert"`
	Body       []byte      `json:"body"`
	Signature  []byte      `json:"signature"`
}

var storageSecret []byte
var storePassphrase []byte
var allowPlaintextIdentity bool
var filelessMode bool

func SetAllowPlaintextIdentity(allow bool) { allowPlaintextIdentity = allow }
func SetFilelessMode(enabled bool)         { filelessMode = enabled }

func SetStorePassphrase(passphrase []byte) {
	storePassphrase = append([]byte(nil), passphrase...)
}

func readFileData(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) > 0 && data[0] == 1 {
		if len(storePassphrase) == 0 {
			return nil, fmt.Errorf("identity file is encrypted; provide --passphrase or set SHROUD_PASSPHRASE")
		}
		plain, err := scrypto.DecryptStore(data, storePassphrase)
		if err != nil {
			return nil, fmt.Errorf("wrong passphrase or corrupted identity file: %w", err)
		}
		return plain, nil
	}
	return data, nil
}

func SetStorageSecret(secret []byte) {
	storageSecret = append([]byte(nil), secret...)
}

func storageDir() string {
	base := ""
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		base = home
	}
	legacy := filepath.Join(base, ".shroud")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	if len(storageSecret) > 0 {
		h := sha256.Sum256(append(storageSecret, []byte("storage-dir")...))
		return filepath.Join(base, "."+hex.EncodeToString(h[:6]))
	}
	return legacy
}

func DefaultAdminPath() string {
	if p := os.Getenv("SHROUD_ADMIN_IDENTITY"); p != "" {
		return p
	}
	return filepath.Join(storageDir(), "admin_identity.json")
}

func DefaultAgentPath() string {
	if p := os.Getenv("SHROUD_AGENT_IDENTITY"); p != "" {
		return p
	}
	return filepath.Join(storageDir(), "agent_identity.json")
}

func LoadOrCreateAdmin(path string) (*AdminStore, error) {
	if path == "" {
		path = DefaultAdminPath()
	}
	st := &AdminStore{Path: path}
	if data, err := readFileData(path); err == nil {
		if err := json.Unmarshal(data, st); err != nil {
			return nil, err
		}
		st.Path = path
		st.initMaps()
		return st, st.ensureAdminMaterial()
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	st.initMaps()
	if err := st.ensureAdminMaterial(); err != nil {
		return nil, err
	}
	return st, st.Save()
}

func LoadOrCreateAgent(path string) (*AgentStore, error) {
	if filelessMode {
		st := &AgentStore{}
		st.initMaps()
		if err := st.ensureAgentMaterial(); err != nil {
			return nil, err
		}
		return st, nil
	}
	if path == "" {
		path = DefaultAgentPath()
	}
	st := &AgentStore{Path: path}
	if data, err := readFileData(path); err == nil {
		if err := json.Unmarshal(data, st); err != nil {
			return nil, err
		}
		st.Path = path
		st.initMaps()
		return st, st.ensureAgentMaterial()
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	st.initMaps()
	if err := st.ensureAgentMaterial(); err != nil {
		return nil, err
	}
	return st, st.Save()
}

func (s *AdminStore) initMaps() {
	if s.AgentCerts == nil {
		s.AgentCerts = make(map[string]Certificate)
	}
	if s.ProtocolUUIDToSerial == nil {
		s.ProtocolUUIDToSerial = make(map[string]string)
	}
	if s.RevokedSerials == nil {
		s.RevokedSerials = make(map[string]int64)
	}
	if s.ConsumedEnrollmentKeys == nil {
		s.ConsumedEnrollmentKeys = make(map[string]int64)
	}
	if s.NextCommandSeq == 0 {
		s.NextCommandSeq = 1
	}
}

func (s *AgentStore) initMaps() {
	if s.RevokedSerials == nil {
		s.RevokedSerials = make(map[string]int64)
	}
	if s.LastCommandSeq == nil {
		s.LastCommandSeq = make(map[string]uint64)
	}
}

func (s *AdminStore) ensureAdminMaterial() error {
	s.initMaps()
	if len(s.CASeed) == 0 {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		s.CASeed = priv.Seed()
	}
	caPriv := ed25519.NewKeyFromSeed(s.CASeed)
	s.CAPublic = caPriv.Public().(ed25519.PublicKey)

	if len(s.NodeSeed) == 0 {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		s.NodeSeed = priv.Seed()
	}
	if len(s.NodeX25519Priv) == 0 {
		priv, _, err := generateX25519()
		if err != nil {
			return err
		}
		s.NodeX25519Priv = priv
	}
	if len(s.NodeCert.Signature) == 0 {
		pub := ed25519.NewKeyFromSeed(s.NodeSeed).Public().(ed25519.PublicKey)
		xpub, err := x25519Public(s.NodeX25519Priv)
		if err != nil {
			return err
		}
		cert := newCertificate(RoleAdmin, "admin", []string{UsageNode}, pub, xpub)
		if err := signCertificate(&cert, caPriv); err != nil {
			return err
		}
		s.NodeCert = cert
	}

	if len(s.CommandSeed) == 0 {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		s.CommandSeed = priv.Seed()
	}
	if len(s.CommandCert.Signature) == 0 {
		pub := ed25519.NewKeyFromSeed(s.CommandSeed).Public().(ed25519.PublicKey)
		cert := newCertificate(RoleCommandSigner, "admin-command", []string{UsageCommandSigner}, pub, nil)
		if err := signCertificate(&cert, caPriv); err != nil {
			return err
		}
		s.CommandCert = cert
	}
	return nil
}

func (s *AgentStore) ensureAgentMaterial() error {
	s.initMaps()
	if len(s.NodeSeed) == 0 {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		s.NodeSeed = priv.Seed()
	}
	if len(s.NodeX25519Priv) == 0 {
		priv, _, err := generateX25519()
		if err != nil {
			return err
		}
		s.NodeX25519Priv = priv
	}
	return nil
}

func (s *AdminStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSONFile(s.Path, s)
}

func (s *AgentStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSONFile(s.Path, s)
}

func wipeBytes(slices ...[]byte) {
	for _, b := range slices {
		for i := range b {
			b[i] = 0
		}
		runtime.KeepAlive(b)
	}
}

func (s *AdminStore) WipeSeeds() {
	s.mu.Lock()
	defer s.mu.Unlock()
	wipeBytes(s.CASeed, s.NodeSeed, s.CommandSeed, s.NodeX25519Priv)
}

func (s *AgentStore) WipeSeeds() {
	s.mu.Lock()
	defer s.mu.Unlock()
	wipeBytes(s.NodeSeed, s.NodeX25519Priv)
}

func ClearAgentIdentity(path string) {
	if path == "" {
		path = DefaultAgentPath()
	}
	os.Remove(path)
}

func writeJSONFile(path string, v any) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if len(storePassphrase) > 0 {
		data, err = scrypto.EncryptStore(data, storePassphrase)
		if err != nil {
			return fmt.Errorf("encrypt identity: %w", err)
		}
	} else if !allowPlaintextIdentity {
		return fmt.Errorf("passphrase required for identity storage (use --passphrase or --identity-plain to override)")
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	marker := filepath.Join(filepath.Dir(path), ".shroud-id")
	_ = os.WriteFile(marker, []byte("Shroud identity store"), 0600)
	return nil
}

func (s *AdminStore) RevokeCert(uuid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	serial, ok := s.ProtocolUUIDToSerial[uuid]
	if !ok {
		return fmt.Errorf("no certificate found for UUID %s", uuid)
	}
	s.RevokedSerials[serial] = time.Now().Unix()
	return nil
}

func (s *AdminStore) ResetEnrollmentKeys() {
	s.mu.Lock()
	s.ConsumedEnrollmentKeys = make(map[string]int64)
	s.mu.Unlock()
}

func (s *AdminStore) NodePrivateKey() ed25519.PrivateKey { return ed25519.NewKeyFromSeed(s.NodeSeed) }
func (s *AgentStore) NodePrivateKey() ed25519.PrivateKey { return ed25519.NewKeyFromSeed(s.NodeSeed) }
func (s *AdminStore) CommandPrivateKey() ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(s.CommandSeed)
}
func (s *AdminStore) HasCertificate() bool { return len(s.NodeCert.Signature) != 0 }
func (s *AgentStore) HasCertificate() bool { return len(s.Cert.Signature) != 0 && len(s.CAPublic) != 0 }

func (s *AgentStore) PublicKeys() (edPub, xPub []byte, err error) {
	if err := s.ensureAgentMaterial(); err != nil {
		return nil, nil, err
	}
	edPub = ed25519.NewKeyFromSeed(s.NodeSeed).Public().(ed25519.PublicKey)
	xPub, err = x25519Public(s.NodeX25519Priv)
	return edPub, xPub, err
}

func (s *AgentStore) ApplyEnrollment(resp EnrollmentResponse) error {
	if err := VerifyCertificate(resp.AgentCert, resp.CAPublic, RoleAgent, UsageNode, resp.RevokedSerials); err != nil {
		return err
	}
	if err := VerifyCertificate(resp.AdminNodeCert, resp.CAPublic, RoleAdmin, UsageNode, resp.RevokedSerials); err != nil {
		return err
	}
	if err := VerifyCertificate(resp.AdminCommandCert, resp.CAPublic, RoleCommandSigner, UsageCommandSigner, resp.RevokedSerials); err != nil {
		return err
	}
	s.Cert = resp.AgentCert
	s.CAPublic = append([]byte(nil), resp.CAPublic...)
	s.AdminNodeCert = resp.AdminNodeCert
	s.AdminCommandCert = resp.AdminCommandCert
	s.RevokedSerials = copyRevoked(resp.RevokedSerials)
	if len(resp.ProtocolMagic) == 4 {
		s.ProtocolMagic = append([]byte(nil), resp.ProtocolMagic...)
	}
	if resp.WebSocketPath != "" {
		s.WebSocketPath = resp.WebSocketPath
	}
	if resp.AdminUUID != "" {
		s.AdminUUID = resp.AdminUUID
	}
	return s.Save()
}

func (s *AdminStore) EnrollAgent(authKey, edPub, xPub []byte) (EnrollmentResponse, error) {
	if len(authKey) == 0 {
		return EnrollmentResponse{}, errors.New("missing enrollment key")
	}
	if len(edPub) != ed25519.PublicKeySize {
		return EnrollmentResponse{}, errors.New("invalid Ed25519 public key")
	}
	if len(xPub) != 32 {
		return EnrollmentResponse{}, errors.New("invalid X25519 public key")
	}
	keyID := EnrollmentKeyID(authKey)
	if _, used := s.ConsumedEnrollmentKeys[keyID]; used && os.Getenv("SHROUD_ALLOW_REENROLL") == "" {
		return EnrollmentResponse{}, errors.New("enrollment token already consumed")
	}
	caPriv := ed25519.NewKeyFromSeed(s.CASeed)
	cert := newCertificate(RoleAgent, randomSerial(), []string{UsageNode}, edPub, xPub)
	if err := signCertificate(&cert, caPriv); err != nil {
		return EnrollmentResponse{}, err
	}
	s.AgentCerts[cert.Serial] = cert
	s.ConsumedEnrollmentKeys[keyID] = time.Now().Unix()
	if err := s.Save(); err != nil {
		return EnrollmentResponse{}, err
	}
	return EnrollmentResponse{
		AgentCert:        cert,
		CAPublic:         append([]byte(nil), s.CAPublic...),
		AdminNodeCert:    s.NodeCert,
		AdminCommandCert: s.CommandCert,
		RevokedSerials:   copyRevoked(s.RevokedSerials),
		ProtocolMagic:    append([]byte(nil), s.ProtocolMagic...),
		WebSocketPath:    s.WebSocketPath,
		AdminUUID:        s.AdminUUID,
	}, nil
}

// IssueAgentCertificate signs a relayed child CSR after the parent link has
// already been authenticated. It intentionally does not consume the bootstrap
// enrollment token: the parent consumed/verified that token on its child link,
// while this method only performs CA issuance and persistence.
func (s *AdminStore) IssueAgentCertificate(edPub, xPub []byte) (EnrollmentResponse, error) {
	if len(edPub) != ed25519.PublicKeySize {
		return EnrollmentResponse{}, errors.New("invalid Ed25519 public key")
	}
	if len(xPub) != 32 {
		return EnrollmentResponse{}, errors.New("invalid X25519 public key")
	}
	caPriv := ed25519.NewKeyFromSeed(s.CASeed)
	cert := newCertificate(RoleAgent, randomSerial(), []string{UsageNode}, edPub, xPub)
	if err := signCertificate(&cert, caPriv); err != nil {
		return EnrollmentResponse{}, err
	}
	s.AgentCerts[cert.Serial] = cert
	if err := s.Save(); err != nil {
		return EnrollmentResponse{}, err
	}
	return EnrollmentResponse{
		AgentCert:        cert,
		CAPublic:         append([]byte(nil), s.CAPublic...),
		AdminNodeCert:    s.NodeCert,
		AdminCommandCert: s.CommandCert,
		RevokedSerials:   copyRevoked(s.RevokedSerials),
		ProtocolMagic:    append([]byte(nil), s.ProtocolMagic...),
		WebSocketPath:    s.WebSocketPath,
		AdminUUID:        s.AdminUUID,
	}, nil
}

func (s *AdminStore) SetProtocolFingerprint(magic []byte, wsPath string) error {
	if len(magic) == 4 {
		s.ProtocolMagic = append([]byte(nil), magic...)
	}
	if wsPath != "" {
		s.WebSocketPath = wsPath
	}
	return s.Save()
}

func (s *AgentStore) SetProtocolFingerprint(magic []byte, wsPath string) error {
	if len(magic) == 4 {
		s.ProtocolMagic = append([]byte(nil), magic...)
	}
	if wsPath != "" {
		s.WebSocketPath = wsPath
	}
	return s.Save()
}

func (s *AdminStore) BindProtocolUUID(uuid string, cert Certificate) error {
	if uuid == "" || len(cert.Signature) == 0 {
		return nil
	}
	s.AgentCerts[cert.Serial] = cert
	s.ProtocolUUIDToSerial[uuid] = cert.Serial
	return s.Save()
}

func (s *AdminStore) CertificateForPeerUUID(peerUUID string) (Certificate, bool) {
	if peerUUID == "" {
		return Certificate{}, false
	}
	serial := s.ProtocolUUIDToSerial[peerUUID]
	if serial == "" {
		serial = peerUUID
	}
	cert, ok := s.AgentCerts[serial]
	return cert, ok
}

func (s *AdminStore) VerifyPeerCertificate(cert Certificate) error {
	return VerifyCertificate(cert, s.CAPublic, RoleAgent, UsageNode, s.RevokedSerials)
}

func (s *AgentStore) VerifyPeerCertificate(cert Certificate) error {
	if err := VerifyCertificate(cert, s.CAPublic, "", UsageNode, s.RevokedSerials); err != nil {
		return err
	}
	if cert.Role != RoleAdmin && cert.Role != RoleAgent {
		return fmt.Errorf("unexpected peer role %q", cert.Role)
	}
	return nil
}

func (s *AdminStore) PayloadKeyForPeerUUID(peerUUID string) []byte {
	if peerUUID == "" {
		return nil
	}
	serial := s.ProtocolUUIDToSerial[peerUUID]
	if serial == "" {
		serial = peerUUID
	}
	cert, ok := s.AgentCerts[serial]
	if !ok {
		return nil
	}
	key, err := DeriveE2EKey(s.NodeX25519Priv, cert.X25519Public, []byte(cert.Serial))
	if err != nil {
		return nil
	}
	return key
}

func (s *AgentStore) PayloadKeyForAdmin() []byte {
	return s.PayloadKeyForPeerCert(s.AdminNodeCert)
}

func (s *AgentStore) PayloadKeyForPeerCert(cert Certificate) []byte {
	if !s.HasCertificate() || len(cert.X25519Public) == 0 {
		return nil
	}
	context := s.Cert.Serial
	if cert.Role == RoleAgent && cert.Serial < context {
		context = cert.Serial
	}
	key, err := DeriveE2EKey(s.NodeX25519Priv, cert.X25519Public, []byte(context))
	if err != nil {
		return nil
	}
	return key
}

func (s *AdminStore) SignCommandPayload(headerAAD, body []byte) ([]byte, error) {
	s.mu.Lock()
	seq := s.NextCommandSeq
	s.NextCommandSeq++
	s.mu.Unlock()

	sp := SignedPayload{
		Version:    1,
		Sequence:   seq,
		Timestamp:  time.Now().Unix(),
		SignerCert: s.CommandCert,
		Body:       append([]byte(nil), body...),
	}
	sp.Signature = ed25519.Sign(s.CommandPrivateKey(), commandSigningBytes(headerAAD, sp))
	if err := s.Save(); err != nil {
		return nil, err
	}
	return json.Marshal(sp)
}

func (s *AgentStore) VerifyCommandPayload(headerAAD, wrapped []byte) ([]byte, error) {
	var sp SignedPayload
	if err := json.Unmarshal(wrapped, &sp); err != nil {
		return nil, err
	}
	if sp.Version != 1 {
		return nil, errors.New("unsupported signed payload version")
	}
	if err := VerifyCertificate(sp.SignerCert, s.CAPublic, RoleCommandSigner, UsageCommandSigner, s.RevokedSerials); err != nil {
		return nil, err
	}
	if time.Since(time.Unix(sp.Timestamp, 0)) > commandTTL || time.Until(time.Unix(sp.Timestamp, 0)) > time.Minute {
		return nil, errors.New("command signature timestamp outside allowed window")
	}
	if !ed25519.Verify(ed25519.PublicKey(sp.SignerCert.Ed25519Public), commandSigningBytes(headerAAD, sp), sp.Signature) {
		return nil, errors.New("invalid command signature")
	}
	key := sp.SignerCert.Serial
	s.mu.Lock()
	last := s.LastCommandSeq[key]
	if sp.Sequence <= last {
		s.mu.Unlock()
		return nil, errors.New("replayed command sequence")
	}
	s.LastCommandSeq[key] = sp.Sequence
	s.mu.Unlock()
	_ = s.Save()
	return sp.Body, nil
}

func VerifyCertificate(cert Certificate, caPub []byte, wantRole, wantUsage string, revoked map[string]int64) error {
	if cert.Version != CertVersion {
		return errors.New("unsupported certificate version")
	}
	if len(cert.Signature) == 0 {
		return ErrNoCertificate
	}
	if len(caPub) != ed25519.PublicKeySize {
		return errors.New("invalid CA public key")
	}
	now := time.Now().Unix()
	if cert.IssuedAt > now+60 || cert.ExpiresAt < now {
		return errors.New("certificate not currently valid")
	}
	if revoked != nil {
		if _, ok := revoked[cert.Serial]; ok {
			return ErrRevoked
		}
	}
	if wantRole != "" && cert.Role != wantRole {
		return fmt.Errorf("unexpected role %q", cert.Role)
	}
	if wantUsage != "" && !hasUsage(cert.Usages, wantUsage) {
		return fmt.Errorf("missing usage %q", wantUsage)
	}
	sig := cert.Signature
	cert.Signature = nil
	data := append([]byte(certSignDomain), mustJSON(cert)...)
	if !ed25519.Verify(ed25519.PublicKey(caPub), data, sig) {
		return errors.New("invalid certificate signature")
	}
	return nil
}

func newCertificate(role, nodeID string, usages []string, edPub, xPub []byte) Certificate {
	now := time.Now()
	return Certificate{
		Version:       CertVersion,
		Serial:        randomSerial(),
		NodeID:        nodeID,
		Role:          role,
		Usages:        append([]string(nil), usages...),
		Ed25519Public: append([]byte(nil), edPub...),
		X25519Public:  append([]byte(nil), xPub...),
		IssuedAt:      now.Unix(),
		ExpiresAt:     now.Add(certTTL).Unix(),
	}
}

func signCertificate(cert *Certificate, caPriv ed25519.PrivateKey) error {
	cert.Signature = nil
	cert.Signature = ed25519.Sign(caPriv, append([]byte(certSignDomain), mustJSON(*cert)...))
	return nil
}

func commandSigningBytes(headerAAD []byte, sp SignedPayload) []byte {
	h := sha256.Sum256(sp.Body)
	buf := make([]byte, 0, len(commandSignDomain)+len(headerAAD)+8+8+len(h))
	buf = append(buf, []byte(commandSignDomain)...)
	buf = append(buf, 0)
	buf = append(buf, headerAAD...)
	var tmp [16]byte
	binary.BigEndian.PutUint64(tmp[:8], sp.Sequence)
	binary.BigEndian.PutUint64(tmp[8:], uint64(sp.Timestamp))
	buf = append(buf, tmp[:]...)
	buf = append(buf, h[:]...)
	return buf
}

func DeriveE2EKey(privBytes, peerPub, context []byte) ([]byte, error) {
	curve := ecdh.X25519()
	priv, err := curve.NewPrivateKey(privBytes)
	if err != nil {
		return nil, err
	}
	pub, err := curve.NewPublicKey(peerPub)
	if err != nil {
		return nil, err
	}
	shared, err := priv.ECDH(pub)
	if err != nil {
		return nil, err
	}
	r := hkdf.New(sha256.New, shared, context, []byte(e2eInfoPrefix))
	out := make([]byte, 32)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, err
	}
	return out, nil
}

func generateX25519() ([]byte, []byte, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv.Bytes(), priv.PublicKey().Bytes(), nil
}

func x25519Public(privBytes []byte) ([]byte, error) {
	priv, err := ecdh.X25519().NewPrivateKey(privBytes)
	if err != nil {
		return nil, err
	}
	return priv.PublicKey().Bytes(), nil
}

func EnrollmentKeyID(authKey []byte) string {
	h := sha256.Sum256(authKey)
	return hex.EncodeToString(h[:])
}

func Fingerprint(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func randomSerial() string {
	b := randomBytes(16)
	return hex.EncodeToString(b)
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		log.Fatalf("crypto/rand unavailable: %v", err)
	}
	return b
}

func hasUsage(usages []string, want string) bool {
	for _, u := range usages {
		if u == want {
			return true
		}
	}
	return false
}

func copyRevoked(in map[string]int64) map[string]int64 {
	out := make(map[string]int64)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		log.Fatalf("json marshal: %v", err)
	}
	return b
}
