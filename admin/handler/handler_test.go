package handler

import (
	"os"
	"path/filepath"
	"testing"

	"Shroud/share"
)

// --- SSH constructor ---

func TestNewSSH(t *testing.T) {
	ssh := NewSSH("192.168.1.1:22")
	if ssh == nil {
		t.Fatal("NewSSH returned nil")
	}
	if ssh.Addr != "192.168.1.1:22" {
		t.Errorf("expected addr '192.168.1.1:22', got '%s'", ssh.Addr)
	}
	if ssh.Method != 0 {
		t.Errorf("expected default method 0, got %d", ssh.Method)
	}
	if ssh.Username != "" {
		t.Errorf("expected empty username, got '%s'", ssh.Username)
	}
	if ssh.Password != "" {
		t.Errorf("expected empty password, got '%s'", ssh.Password)
	}
	if ssh.Certificate != nil {
		t.Error("expected nil certificate")
	}
}

func TestNewSSH_FieldAssignment(t *testing.T) {
	ssh := NewSSH("10.0.0.1:2222")
	ssh.Method = CERMETHOD
	ssh.Username = "root"
	ssh.Password = "pass"
	ssh.HostKeyFingerprint = "SHA256:abc"

	if ssh.Method != CERMETHOD {
		t.Errorf("expected method CERMETHOD(%d), got %d", CERMETHOD, ssh.Method)
	}
	if ssh.Username != "root" {
		t.Errorf("expected username 'root', got '%s'", ssh.Username)
	}
	if ssh.HostKeyFingerprint != "SHA256:abc" {
		t.Errorf("expected fingerprint 'SHA256:abc', got '%s'", ssh.HostKeyFingerprint)
	}
}

// --- SSHTunnel constructor ---

func TestNewSSHTunnel(t *testing.T) {
	tunnel := NewSSHTunnel("8080", "192.168.1.1:22")
	if tunnel == nil {
		t.Fatal("NewSSHTunnel returned nil")
	}
	if tunnel.Port != "8080" {
		t.Errorf("expected port '8080', got '%s'", tunnel.Port)
	}
	if tunnel.Addr != "192.168.1.1:22" {
		t.Errorf("expected addr '192.168.1.1:22', got '%s'", tunnel.Addr)
	}
	if tunnel.Method != 0 {
		t.Errorf("expected default method 0, got %d", tunnel.Method)
	}
	if tunnel.Username != "" {
		t.Errorf("expected empty username, got '%s'", tunnel.Username)
	}
}

func TestNewSSHTunnel_FieldAssignment(t *testing.T) {
	tunnel := NewSSHTunnel("9090", "10.0.0.1:22")
	tunnel.Method = UPMETHOD
	tunnel.Username = "admin"
	tunnel.Password = "secret"
	tunnel.HostKeyFingerprint = "SHA256:xyz"

	if tunnel.Method != UPMETHOD {
		t.Errorf("expected method UPMETHOD(%d), got %d", UPMETHOD, tunnel.Method)
	}
	if tunnel.Password != "secret" {
		t.Errorf("expected password 'secret', got '%s'", tunnel.Password)
	}
}

// --- getCertificate ---

func TestSSH_getCertificate_ValidFile(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "test_key.pem")

	content := []byte("-----BEGIN RSA PRIVATE KEY-----\nfake-key-data\n-----END RSA PRIVATE KEY-----\n")
	if err := os.WriteFile(certPath, content, 0600); err != nil {
		t.Fatalf("failed to write temp cert: %v", err)
	}

	ssh := NewSSH("192.168.1.1:22")
	ssh.CertificatePath = certPath

	err := ssh.getCertificate()
	if err != nil {
		t.Fatalf("getCertificate failed: %v", err)
	}
	if string(ssh.Certificate) != string(content) {
		t.Error("certificate content does not match written file")
	}
}

func TestSSH_getCertificate_MissingFile(t *testing.T) {
	ssh := NewSSH("192.168.1.1:22")
	ssh.CertificatePath = "/nonexistent/path/key.pem"

	err := ssh.getCertificate()
	if err == nil {
		t.Fatal("expected error for missing certificate file")
	}
}

func TestSSHTunnel_getCertificate_ValidFile(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tunnel_key.pem")

	content := []byte("tunnel-key-content")
	if err := os.WriteFile(certPath, content, 0600); err != nil {
		t.Fatalf("failed to write temp cert: %v", err)
	}

	tunnel := NewSSHTunnel("8080", "10.0.0.1:22")
	tunnel.CertificatePath = certPath

	err := tunnel.getCertificate()
	if err != nil {
		t.Fatalf("getCertificate failed: %v", err)
	}
	if string(tunnel.Certificate) != string(content) {
		t.Error("certificate content does not match written file")
	}
}

func TestSSHTunnel_getCertificate_MissingFile(t *testing.T) {
	tunnel := NewSSHTunnel("8080", "10.0.0.1:22")
	tunnel.CertificatePath = "/nonexistent/path/key.pem"

	err := tunnel.getCertificate()
	if err == nil {
		t.Fatal("expected error for missing certificate file")
	}
}

func TestSSH_getCertificate_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "empty_key.pem")

	if err := os.WriteFile(certPath, []byte{}, 0600); err != nil {
		t.Fatalf("failed to write temp cert: %v", err)
	}

	ssh := NewSSH("192.168.1.1:22")
	ssh.CertificatePath = certPath

	err := ssh.getCertificate()
	if err != nil {
		t.Fatalf("getCertificate failed on empty file: %v", err)
	}
	if len(ssh.Certificate) != 0 {
		t.Errorf("expected empty certificate, got %d bytes", len(ssh.Certificate))
	}
}

// --- NewBar ---

func TestNewBar(t *testing.T) {
	bar := NewBar(1024)
	if bar == nil {
		t.Fatal("NewBar returned nil")
	}
}

// --- StartBar ---

func TestStartBar(t *testing.T) {
	statusChan := make(chan *share.Status)
	done := make(chan struct{})

	go func() {
		StartBar(statusChan, 100)
		close(done)
	}()

	statusChan <- &share.Status{Stat: share.START}
	statusChan <- &share.Status{Stat: share.ADD, Scale: 50}
	statusChan <- &share.Status{Stat: share.ADD, Scale: 50}
	statusChan <- &share.Status{Stat: share.DONE}

	<-done
}

// --- Method constants ---

func TestMethodConstants(t *testing.T) {
	if UPMETHOD != 0 {
		t.Errorf("UPMETHOD expected 0, got %d", UPMETHOD)
	}
	if CERMETHOD != 1 {
		t.Errorf("CERMETHOD expected 1, got %d", CERMETHOD)
	}
}
