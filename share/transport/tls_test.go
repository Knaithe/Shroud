package transport

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"net"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// newRandomTLSKeyPair
// ---------------------------------------------------------------------------

func TestNewRandomTLSKeyPair_ReturnsValidCert(t *testing.T) {
	cert, err := newRandomTLSKeyPair()
	if err != nil {
		t.Fatalf("newRandomTLSKeyPair() error: %v", err)
	}
	if cert == nil {
		t.Fatal("expected non-nil certificate")
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("certificate chain is empty")
	}
	if cert.PrivateKey == nil {
		t.Fatal("private key is nil")
	}
}

func TestNewRandomTLSKeyPair_UniqueCerts(t *testing.T) {
	cert1, err := newRandomTLSKeyPair()
	if err != nil {
		t.Fatalf("first keypair error: %v", err)
	}
	cert2, err := newRandomTLSKeyPair()
	if err != nil {
		t.Fatalf("second keypair error: %v", err)
	}
	fp1 := certFingerprint(cert1)
	fp2 := certFingerprint(cert2)
	if fp1 == fp2 {
		t.Fatal("two generated certificates must have different fingerprints")
	}
}

// ---------------------------------------------------------------------------
// NewServerTLSConfig
// ---------------------------------------------------------------------------

func TestNewServerTLSConfig_MinVersion(t *testing.T) {
	cfg, err := NewServerTLSConfig()
	if err != nil {
		t.Fatalf("NewServerTLSConfig() error: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion = %d, want %d (TLS 1.3)", cfg.MinVersion, tls.VersionTLS13)
	}
}

func TestNewServerTLSConfig_HasCertificate(t *testing.T) {
	cfg, err := NewServerTLSConfig()
	if err != nil {
		t.Fatalf("NewServerTLSConfig() error: %v", err)
	}
	if len(cfg.Certificates) == 0 {
		t.Fatal("server TLS config has no certificates")
	}
	if len(cfg.Certificates[0].Certificate) == 0 {
		t.Fatal("server certificate chain is empty")
	}
}

// ---------------------------------------------------------------------------
// certFingerprint
// ---------------------------------------------------------------------------

func TestCertFingerprint_HexSHA256(t *testing.T) {
	cert, err := newRandomTLSKeyPair()
	if err != nil {
		t.Fatalf("keypair error: %v", err)
	}
	fp := certFingerprint(cert)

	// SHA256 produces 32 bytes = 64 hex characters
	if len(fp) != 64 {
		t.Fatalf("fingerprint length = %d, want 64 hex chars", len(fp))
	}

	// Must be valid lowercase hex
	for _, c := range fp {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("fingerprint contains non-hex character: %c", c)
		}
	}

	// Verify against manual computation
	h := sha256.Sum256(cert.Certificate[0])
	expected := hex.EncodeToString(h[:])
	if fp != expected {
		t.Fatalf("fingerprint mismatch: got %s, want %s", fp, expected)
	}
}

func TestCertFingerprint_Deterministic(t *testing.T) {
	cert, err := newRandomTLSKeyPair()
	if err != nil {
		t.Fatalf("keypair error: %v", err)
	}
	fp1 := certFingerprint(cert)
	fp2 := certFingerprint(cert)
	if fp1 != fp2 {
		t.Fatal("certFingerprint must be deterministic for the same cert")
	}
}

// ---------------------------------------------------------------------------
// NewClientTLSConfig
// ---------------------------------------------------------------------------

func TestNewClientTLSConfig_EmptyFingerprint_DefaultAcceptsAndPrints(t *testing.T) {
	cfg, err := NewClientTLSConfig("example.com", "", false)
	if err != nil {
		t.Fatalf("unexpected error with empty fingerprint: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config with empty fingerprint")
	}
}

func TestNewClientTLSConfig_EmptyFingerprint_Insecure_TOFU(t *testing.T) {
	cfg, err := NewClientTLSConfig("example.com", "", true)
	if err != nil {
		t.Fatalf("unexpected error with deprecated insecure flag: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config with deprecated insecure flag")
	}
}

func TestNewClientTLSConfig_WithFingerprint(t *testing.T) {
	fp := strings.Repeat("ab", 32) // 64-char fake fingerprint
	cfg, err := NewClientTLSConfig("example.com", fp, false)
	if err != nil {
		t.Fatalf("unexpected error with valid fingerprint: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config with fingerprint")
	}
}

func TestNewClientTLSConfig_MinVersion(t *testing.T) {
	fp := strings.Repeat("ab", 32)
	cfg, err := NewClientTLSConfig("example.com", fp, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Fatalf("MinVersion = %d, want %d (TLS 1.3)", cfg.MinVersion, tls.VersionTLS13)
	}
}

func TestNewClientTLSConfig_InsecureSkipVerify(t *testing.T) {
	fp := strings.Repeat("ab", 32)
	cfg, err := NewClientTLSConfig("example.com", fp, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify should be true (manual fingerprint verification)")
	}
}

// ---------------------------------------------------------------------------
// VerifyPeerCertificate callback
// ---------------------------------------------------------------------------

func TestVerifyPeerCertificate_MatchingFingerprint(t *testing.T) {
	cert, err := newRandomTLSKeyPair()
	if err != nil {
		t.Fatalf("keypair error: %v", err)
	}
	fp := certFingerprint(cert)

	cfg, err := NewClientTLSConfig("example.com", fp, false)
	if err != nil {
		t.Fatalf("client config error: %v", err)
	}

	err = cfg.VerifyPeerCertificate([][]byte{cert.Certificate[0]}, nil)
	if err != nil {
		t.Fatalf("VerifyPeerCertificate should pass with matching fingerprint: %v", err)
	}
}

func TestVerifyPeerCertificate_MismatchedFingerprint(t *testing.T) {
	cert, err := newRandomTLSKeyPair()
	if err != nil {
		t.Fatalf("keypair error: %v", err)
	}
	wrongFP := strings.Repeat("00", 32)

	cfg, err := NewClientTLSConfig("example.com", wrongFP, false)
	if err != nil {
		t.Fatalf("client config error: %v", err)
	}

	err = cfg.VerifyPeerCertificate([][]byte{cert.Certificate[0]}, nil)
	if err == nil {
		t.Fatal("VerifyPeerCertificate should fail with mismatched fingerprint")
	}
	if !strings.Contains(err.Error(), "fingerprint mismatch") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestVerifyPeerCertificate_EmptyFingerprintMode(t *testing.T) {
	cert, err := newRandomTLSKeyPair()
	if err != nil {
		t.Fatalf("keypair error: %v", err)
	}

	cfg, err := NewClientTLSConfig("example.com", "", false)
	if err != nil {
		t.Fatalf("client config error: %v", err)
	}

	// Empty fingerprint mode accepts the certificate and prints its fingerprint.
	err = cfg.VerifyPeerCertificate([][]byte{cert.Certificate[0]}, nil)
	if err != nil {
		t.Fatalf("VerifyPeerCertificate should pass with empty fingerprint: %v", err)
	}
}

func TestVerifyPeerCertificate_NoCerts(t *testing.T) {
	fp := strings.Repeat("ab", 32)
	cfg, err := NewClientTLSConfig("example.com", fp, false)
	if err != nil {
		t.Fatalf("client config error: %v", err)
	}

	err = cfg.VerifyPeerCertificate([][]byte{}, nil)
	if err == nil {
		t.Fatal("VerifyPeerCertificate should fail when no certs are presented")
	}
	if !strings.Contains(err.Error(), "no server certificate") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// End-to-end TLS handshake
// ---------------------------------------------------------------------------

func TestTLSHandshake_EndToEnd(t *testing.T) {
	serverCfg, err := NewServerTLSConfig()
	if err != nil {
		t.Fatalf("server config error: %v", err)
	}

	// Extract the fingerprint from the server certificate
	fp := certFingerprint(&serverCfg.Certificates[0])

	clientCfg, err := NewClientTLSConfig("localhost", fp, false)
	if err != nil {
		t.Fatalf("client config error: %v", err)
	}

	// Create an in-memory connection pair
	rawClient, rawServer := net.Pipe()

	var wg sync.WaitGroup
	var serverErr, clientErr error

	wg.Add(2)

	// Server side
	go func() {
		defer wg.Done()
		tlsServer := tls.Server(rawServer, serverCfg)
		defer tlsServer.Close()
		if err := tlsServer.Handshake(); err != nil {
			serverErr = err
			return
		}
		// Echo one message to prove the connection works
		buf := make([]byte, 64)
		n, err := tlsServer.Read(buf)
		if err != nil {
			serverErr = err
			return
		}
		if _, err := tlsServer.Write(buf[:n]); err != nil {
			serverErr = err
		}
	}()

	// Client side
	go func() {
		defer wg.Done()
		tlsClient := tls.Client(rawClient, clientCfg)
		defer tlsClient.Close()
		if err := tlsClient.Handshake(); err != nil {
			clientErr = err
			return
		}
		msg := []byte("hello-tls")
		if _, err := tlsClient.Write(msg); err != nil {
			clientErr = err
			return
		}
		buf := make([]byte, 64)
		n, err := tlsClient.Read(buf)
		if err != nil {
			clientErr = err
			return
		}
		if string(buf[:n]) != string(msg) {
			clientErr = err
		}
	}()

	wg.Wait()

	if serverErr != nil {
		t.Fatalf("server handshake/IO error: %v", serverErr)
	}
	if clientErr != nil {
		t.Fatalf("client handshake/IO error: %v", clientErr)
	}
}

func TestTLSHandshake_MismatchedFingerprint_Fails(t *testing.T) {
	serverCfg, err := NewServerTLSConfig()
	if err != nil {
		t.Fatalf("server config error: %v", err)
	}

	// Use a wrong fingerprint
	wrongFP := strings.Repeat("ff", 32)
	clientCfg, err := NewClientTLSConfig("localhost", wrongFP, false)
	if err != nil {
		t.Fatalf("client config error: %v", err)
	}

	rawClient, rawServer := net.Pipe()

	var wg sync.WaitGroup
	var clientErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		tlsServer := tls.Server(rawServer, serverCfg)
		defer tlsServer.Close()
		// Server handshake may or may not fail depending on timing
		tlsServer.Handshake()
	}()

	go func() {
		defer wg.Done()
		tlsClient := tls.Client(rawClient, clientCfg)
		defer tlsClient.Close()
		clientErr = tlsClient.Handshake()
	}()

	wg.Wait()

	if clientErr == nil {
		t.Fatal("expected client handshake to fail with mismatched fingerprint")
	}
}
