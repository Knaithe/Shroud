package share

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"net"
	"sync"
	"testing"
	"time"

	"Shroud/crypto"
	"Shroud/identity"
)

func init() {
	identity.SetAllowPlaintextIdentity(true)
}

func TestTokenGenerationConsistency(t *testing.T) {
	GeneratePreAuthToken([]byte("secret-A"))
	keyA1 := append([]byte(nil), AuthKey...)
	GeneratePreAuthToken([]byte("secret-A"))
	keyA2 := append([]byte(nil), AuthKey...)
	if !bytes.Equal(keyA1, keyA2) {
		t.Fatal("same secret produced different keys")
	}
	GeneratePreAuthToken([]byte("secret-B"))
	keyB := append([]byte(nil), AuthKey...)
	if bytes.Equal(keyA1, keyB) {
		t.Fatal("different secrets produced the same key")
	}
}

func TestTokenGenerationDeterministic(t *testing.T) {
	secret := "my-test-secret"
	GeneratePreAuthToken([]byte(secret))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(crypto.PurposeAuth))
	expected := mac.Sum(nil)
	if !bytes.Equal(AuthKey, expected) {
		t.Fatalf("AuthKey mismatch\ngot:  %x\nwant: %x", AuthKey, expected)
	}
}

func TestFingerprintFromAuthKeyStableAndConfigurable(t *testing.T) {
	GeneratePreAuthToken([]byte("fingerprint-secret"))
	magic1, path1 := FingerprintFromAuthKey(AuthKey)
	magic2, path2 := FingerprintFromAuthKey(AuthKey)
	if len(magic1) != 4 || path1 == "" || path1[0] != '/' {
		t.Fatalf("bad fingerprint magic=%x path=%q", magic1, path1)
	}
	if !bytes.Equal(magic1, magic2) || path1 != path2 {
		t.Fatal("fingerprint derivation is not deterministic")
	}
	if err := SetMagic([]byte("ABCD")); err != nil {
		t.Fatalf("SetMagic: %v", err)
	}
	if !bytes.Equal(Magic(), []byte("ABCD")) {
		t.Fatalf("magic was not updated: %q", Magic())
	}
}

func TestPassiveAdminRejectsInvalidMagic(t *testing.T) {
	GeneratePreAuthToken([]byte("test-secret"))
	admin, err := identity.LoadOrCreateAdmin(t.TempDir() + "/admin.json")
	if err != nil {
		t.Fatalf("admin identity: %v", err)
	}
	if err := SetMagic([]byte("GOOD")); err != nil {
		t.Fatalf("SetMagic: %v", err)
	}

	client, server := net.Pipe()
	var wg sync.WaitGroup
	var passiveErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _, passiveErr = PassiveAdminAuthAndExchange(server, admin)
	}()
	go func() {
		defer wg.Done()
		_, _ = client.Write([]byte("BADD"))
		client.Close()
	}()
	wg.Wait()
	if passiveErr == nil {
		t.Fatal("expected invalid magic to be rejected")
	}
}

func TestEnrollmentThenCertReconnectWithoutAuthKey(t *testing.T) {
	GeneratePreAuthToken([]byte("bootstrap-once"))
	admin, err := identity.LoadOrCreateAdmin(t.TempDir() + "/admin.json")
	if err != nil {
		t.Fatalf("admin identity: %v", err)
	}
	agent, err := identity.LoadOrCreateAgent(t.TempDir() + "/agent.json")
	if err != nil {
		t.Fatalf("agent identity: %v", err)
	}
	if err := SetMagic([]byte("ENR1")); err != nil {
		t.Fatalf("SetMagic: %v", err)
	}

	client, server := net.Pipe()
	var wg sync.WaitGroup
	var activeKey, passiveKey []byte
	var activeErr, passiveErr error
	var peerCert identity.Certificate
	wg.Add(2)
	go func() {
		defer wg.Done()
		activeKey, activeErr = ActiveAgentAuthAndExchange(client, agent)
	}()
	go func() {
		defer wg.Done()
		passiveKey, peerCert, passiveErr = PassiveAdminAuthAndExchange(server, admin)
	}()
	wg.Wait()
	if activeErr != nil || passiveErr != nil {
		t.Fatalf("enrollment errors active=%v passive=%v", activeErr, passiveErr)
	}
	if !bytes.Equal(activeKey, passiveKey) {
		t.Fatal("enrollment link keys differ")
	}
	if !agent.HasCertificate() || len(peerCert.Signature) == 0 {
		t.Fatal("enrollment did not issue/persist cert")
	}

	ClearPreAuthToken()
	client, server = net.Pipe()
	activeKey, passiveKey = nil, nil
	activeErr, passiveErr = nil, nil
	wg.Add(2)
	go func() {
		defer wg.Done()
		activeKey, activeErr = ActiveAgentAuthAndExchange(client, agent)
	}()
	go func() {
		defer wg.Done()
		passiveKey, peerCert, passiveErr = PassiveAdminAuthAndExchange(server, admin)
	}()
	wg.Wait()
	if activeErr != nil || passiveErr != nil {
		t.Fatalf("cert reconnect errors active=%v passive=%v", activeErr, passiveErr)
	}
	if !bytes.Equal(activeKey, passiveKey) {
		t.Fatal("cert reconnect link keys differ")
	}
}

func TestAdminActiveIssuesCertificateToPassiveAgent(t *testing.T) {
	GeneratePreAuthToken([]byte("admin-active-bootstrap"))
	admin, err := identity.LoadOrCreateAdmin(t.TempDir() + "/admin.json")
	if err != nil {
		t.Fatalf("admin identity: %v", err)
	}
	agent, err := identity.LoadOrCreateAgent(t.TempDir() + "/agent.json")
	if err != nil {
		t.Fatalf("agent identity: %v", err)
	}
	if err := SetMagic([]byte("ENR2")); err != nil {
		t.Fatalf("SetMagic: %v", err)
	}

	client, server := net.Pipe()
	var wg sync.WaitGroup
	var activeKey, passiveKey []byte
	var activeErr, passiveErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		activeKey, _, activeErr = ActiveAdminIssueEnrollAndExchange(client, admin)
	}()
	go func() {
		defer wg.Done()
		passiveKey, _, passiveErr = PassiveAgentAuthAndExchange(server, agent)
	}()
	wg.Wait()
	if activeErr != nil || passiveErr != nil {
		t.Fatalf("issue enrollment errors active=%v passive=%v", activeErr, passiveErr)
	}
	if !bytes.Equal(activeKey, passiveKey) {
		t.Fatal("issued enrollment link keys differ")
	}
	if !agent.HasCertificate() {
		t.Fatal("passive agent did not persist issued cert")
	}
}

func TestRelayedChildEnrollmentViaParentAgent(t *testing.T) {
	GeneratePreAuthToken([]byte("child-bootstrap"))
	admin, err := identity.LoadOrCreateAdmin(t.TempDir() + "/admin.json")
	if err != nil {
		t.Fatalf("admin identity: %v", err)
	}
	parent, err := identity.LoadOrCreateAgent(t.TempDir() + "/parent.json")
	if err != nil {
		t.Fatalf("parent identity: %v", err)
	}
	child, err := identity.LoadOrCreateAgent(t.TempDir() + "/child.json")
	if err != nil {
		t.Fatalf("child identity: %v", err)
	}
	if err := SetMagic([]byte("ENR3")); err != nil {
		t.Fatalf("SetMagic: %v", err)
	}
	edPub, xPub, _ := parent.PublicKeys()
	resp, err := admin.EnrollAgent(AuthKey, edPub, xPub)
	if err != nil {
		t.Fatalf("parent enroll: %v", err)
	}
	if err := parent.ApplyEnrollment(resp); err != nil {
		t.Fatalf("parent apply: %v", err)
	}

	client, server := net.Pipe()
	var wg sync.WaitGroup
	var activeKey, passiveKey []byte
	var activeErr, passiveErr error
	relay := func(edPub, xPub []byte) (identity.EnrollmentResponse, error) {
		return admin.IssueAgentCertificate(edPub, xPub)
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		activeKey, activeErr = ActiveAgentAuthAndExchange(client, child)
	}()
	go func() {
		defer wg.Done()
		passiveKey, _, passiveErr = PassiveAgentEnrollRelayOrCertAndExchange(server, parent, relay)
	}()
	wg.Wait()
	if activeErr != nil || passiveErr != nil {
		t.Fatalf("relayed enrollment errors active=%v passive=%v", activeErr, passiveErr)
	}
	if !bytes.Equal(activeKey, passiveKey) {
		t.Fatal("relayed enrollment link keys differ")
	}
	if !child.HasCertificate() {
		t.Fatal("child did not persist relayed certificate")
	}
	if err := admin.VerifyPeerCertificate(child.Cert); err != nil {
		t.Fatalf("admin cannot verify child cert: %v", err)
	}
}

func TestRevokedCertificateRejectedOnReconnect(t *testing.T) {
	GeneratePreAuthToken([]byte("revoke-bootstrap"))
	admin, err := identity.LoadOrCreateAdmin(t.TempDir() + "/admin.json")
	if err != nil {
		t.Fatalf("admin identity: %v", err)
	}
	agent, err := identity.LoadOrCreateAgent(t.TempDir() + "/agent.json")
	if err != nil {
		t.Fatalf("agent identity: %v", err)
	}
	if err := SetMagic([]byte("RVK1")); err != nil {
		t.Fatalf("SetMagic: %v", err)
	}
	edPub, xPub, _ := agent.PublicKeys()
	resp, err := admin.EnrollAgent(AuthKey, edPub, xPub)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if err := agent.ApplyEnrollment(resp); err != nil {
		t.Fatalf("apply: %v", err)
	}
	admin.RevokedSerials[agent.Cert.Serial] = time.Now().Unix()

	client, server := net.Pipe()
	var wg sync.WaitGroup
	var activeErr, passiveErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, activeErr = ActiveAgentAuthAndExchange(client, agent)
	}()
	go func() {
		defer wg.Done()
		_, _, passiveErr = PassiveAdminAuthAndExchange(server, admin)
	}()
	wg.Wait()
	if passiveErr == nil {
		t.Fatal("expected revoked cert to be rejected by admin")
	}
	if activeErr == nil {
		t.Fatal("expected active side to observe revocation failure")
	}
}
