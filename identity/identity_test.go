package identity

import (
	"bytes"
	"testing"
)

func init() {
	allowPlaintextIdentity = true
}

func TestEnrollAgentConsumesTokenAndIssuesUsableCert(t *testing.T) {
	admin, err := LoadOrCreateAdmin(t.TempDir() + "/admin.json")
	if err != nil {
		t.Fatalf("admin identity: %v", err)
	}
	agent, err := LoadOrCreateAgent(t.TempDir() + "/agent.json")
	if err != nil {
		t.Fatalf("agent identity: %v", err)
	}
	edPub, xPub, err := agent.PublicKeys()
	if err != nil {
		t.Fatalf("agent public keys: %v", err)
	}
	tokenKey := []byte("derived-enrollment-key-32-bytes!!")
	resp, err := admin.EnrollAgent(tokenKey, edPub, xPub)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if err := agent.ApplyEnrollment(resp); err != nil {
		t.Fatalf("apply enrollment: %v", err)
	}
	if !agent.HasCertificate() {
		t.Fatal("agent should have a certificate after enrollment")
	}
	if err := admin.VerifyPeerCertificate(agent.Cert); err != nil {
		t.Fatalf("admin verify agent cert: %v", err)
	}
	if _, err := admin.EnrollAgent(tokenKey, edPub, xPub); err == nil {
		t.Fatal("expected one-time enrollment token reuse to fail")
	}
}

func TestE2EKeyMatchesForAdminAndAgent(t *testing.T) {
	admin, err := LoadOrCreateAdmin(t.TempDir() + "/admin.json")
	if err != nil {
		t.Fatalf("admin identity: %v", err)
	}
	agent, err := LoadOrCreateAgent(t.TempDir() + "/agent.json")
	if err != nil {
		t.Fatalf("agent identity: %v", err)
	}
	edPub, xPub, _ := agent.PublicKeys()
	resp, err := admin.EnrollAgent([]byte("another-token-key"), edPub, xPub)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if err := agent.ApplyEnrollment(resp); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := admin.BindProtocolUUID("RUNTIME001", agent.Cert); err != nil {
		t.Fatalf("bind: %v", err)
	}
	adminKey := admin.PayloadKeyForPeerUUID("RUNTIME001")
	agentKey := agent.PayloadKeyForAdmin()
	if len(adminKey) != 32 || len(agentKey) != 32 {
		t.Fatalf("bad key lengths admin=%d agent=%d", len(adminKey), len(agentKey))
	}
	if !bytes.Equal(adminKey, agentKey) {
		t.Fatal("admin and agent E2E keys differ")
	}
}

func TestCommandSignatureRejectsTamperAndReplay(t *testing.T) {
	admin, err := LoadOrCreateAdmin(t.TempDir() + "/admin.json")
	if err != nil {
		t.Fatalf("admin identity: %v", err)
	}
	agent, err := LoadOrCreateAgent(t.TempDir() + "/agent.json")
	if err != nil {
		t.Fatalf("agent identity: %v", err)
	}
	edPub, xPub, _ := agent.PublicKeys()
	resp, err := admin.EnrollAgent([]byte("cmd-token-key"), edPub, xPub)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if err := agent.ApplyEnrollment(resp); err != nil {
		t.Fatalf("apply: %v", err)
	}

	aad := []byte("header-aad")
	wrapped, err := admin.SignCommandPayload(aad, []byte("whoami"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	plain, err := agent.VerifyCommandPayload(aad, wrapped)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if string(plain) != "whoami" {
		t.Fatalf("plain = %q", plain)
	}
	if _, err := agent.VerifyCommandPayload(aad, wrapped); err == nil {
		t.Fatal("expected replay to be rejected")
	}

	wrapped2, _ := admin.SignCommandPayload(aad, []byte("id"))
	wrapped2[len(wrapped2)-1] ^= 1
	if _, err := agent.VerifyCommandPayload(aad, wrapped2); err == nil {
		t.Fatal("expected tampered payload to be rejected")
	}
}
