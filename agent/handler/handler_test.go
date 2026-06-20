package handler

import (
	"testing"
)

func TestNewSSH(t *testing.T) {
	s := newSSH()
	if s == nil {
		t.Fatal("newSSH returned nil")
	}
	// Zero-value fields
	if s.Addr != "" {
		t.Fatalf("expected empty Addr, got %q", s.Addr)
	}
	if s.Username != "" {
		t.Fatalf("expected empty Username, got %q", s.Username)
	}
	if s.Password != "" {
		t.Fatalf("expected empty Password, got %q", s.Password)
	}
	if s.Method != 0 {
		t.Fatalf("expected Method 0, got %d", s.Method)
	}
	if s.Certificate != nil {
		t.Fatal("expected nil Certificate")
	}
	if s.HostKeyFingerprint != "" {
		t.Fatalf("expected empty HostKeyFingerprint, got %q", s.HostKeyFingerprint)
	}
}

func TestNewSSHTunnel(t *testing.T) {
	cert := []byte("fake-cert-data")
	tunnel := newSSHTunnel(CERMETHOD, "10.0.0.1:22", "8080", "user", "pass", cert, "SHA256:abc")
	if tunnel == nil {
		t.Fatal("newSSHTunnel returned nil")
	}
	if tunnel.Method != CERMETHOD {
		t.Fatalf("expected Method %d, got %d", CERMETHOD, tunnel.Method)
	}
	if tunnel.Addr != "10.0.0.1:22" {
		t.Fatalf("expected Addr '10.0.0.1:22', got %q", tunnel.Addr)
	}
	if tunnel.Port != "8080" {
		t.Fatalf("expected Port '8080', got %q", tunnel.Port)
	}
	if tunnel.Username != "user" {
		t.Fatalf("expected Username 'user', got %q", tunnel.Username)
	}
	if tunnel.Password != "pass" {
		t.Fatalf("expected Password 'pass', got %q", tunnel.Password)
	}
	if string(tunnel.Certificate) != "fake-cert-data" {
		t.Fatalf("expected Certificate 'fake-cert-data', got %q", string(tunnel.Certificate))
	}
	if tunnel.HostKeyFingerprint != "SHA256:abc" {
		t.Fatalf("expected HostKeyFingerprint 'SHA256:abc', got %q", tunnel.HostKeyFingerprint)
	}
}

func TestNewSSHTunnel_UPMethod(t *testing.T) {
	tunnel := newSSHTunnel(UPMETHOD, "192.168.1.1:22", "3306", "admin", "secret", nil, "")
	if tunnel.Method != UPMETHOD {
		t.Fatalf("expected UPMETHOD (%d), got %d", UPMETHOD, tunnel.Method)
	}
	if tunnel.Certificate != nil {
		t.Fatal("expected nil Certificate for UPMETHOD")
	}
	if tunnel.HostKeyFingerprint != "" {
		t.Fatalf("expected empty HostKeyFingerprint, got %q", tunnel.HostKeyFingerprint)
	}
}

func TestNewSocks(t *testing.T) {
	s := newSocks()
	if s == nil {
		t.Fatal("newSocks returned nil")
	}
	if s.Username != "" {
		t.Fatalf("expected empty Username, got %q", s.Username)
	}
	if s.Password != "" {
		t.Fatalf("expected empty Password, got %q", s.Password)
	}
}

func TestSettingDefaults(t *testing.T) {
	s := new(Setting)
	if s.method != "" {
		t.Fatalf("expected empty method, got %q", s.method)
	}
	if s.isAuthed {
		t.Fatal("expected isAuthed false")
	}
	if s.tcpConnected {
		t.Fatal("expected tcpConnected false")
	}
	if s.isUDP {
		t.Fatal("expected isUDP false")
	}
	if s.success {
		t.Fatal("expected success false")
	}
	if s.tcpConn != nil {
		t.Fatal("expected nil tcpConn")
	}
	if s.udpListener != nil {
		t.Fatal("expected nil udpListener")
	}
}

func TestSSHMethodConstants(t *testing.T) {
	if UPMETHOD != 0 {
		t.Fatalf("expected UPMETHOD=0, got %d", UPMETHOD)
	}
	if CERMETHOD != 1 {
		t.Fatalf("expected CERMETHOD=1, got %d", CERMETHOD)
	}
}
