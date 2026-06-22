package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
)

func newRandomTLSKeyPair() (*tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&key.PublicKey,
		key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &tlsCert, nil
}

var tlsCertPath string

func SetTLSCertPath(path string) { tlsCertPath = path }

func loadOrCreateTLSKeyPair() (*tls.Certificate, error) {
	if tlsCertPath != "" {
		certFile := tlsCertPath + ".crt"
		keyFile := tlsCertPath + ".key"
		if _, err := os.Stat(certFile); err == nil {
			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err == nil {
				return &cert, nil
			}
		}
		cert, err := newRandomTLSKeyPair()
		if err != nil {
			return nil, err
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(cert.PrivateKey.(*rsa.PrivateKey))})
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
		_ = os.WriteFile(certFile, certPEM, 0600)
		_ = os.WriteFile(keyFile, keyPEM, 0600)
		return cert, nil
	}
	return newRandomTLSKeyPair()
}

func NewServerTLSConfig() (*tls.Config, error) {
	cert, err := loadOrCreateTLSKeyPair()
	if err != nil {
		return nil, err
	}

	base := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS13,
	}
	fmt.Fprintf(os.Stderr, "[*] TLS certificate fingerprint (SHA256): %s\n", certFingerprint(cert))
	return base, nil
}

func certFingerprint(cert *tls.Certificate) string {
	h := sha256.Sum256(cert.Certificate[0])
	return hex.EncodeToString(h[:])
}

func NewClientTLSConfig(serverName string, expectedFingerprint string, insecure bool) (*tls.Config, error) {
	if expectedFingerprint == "" && !insecure {
		return nil, fmt.Errorf("TLS requires --tls-fingerprint or --tls-insecure")
	}

	base := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         serverName,
		MinVersion:         tls.VersionTLS13,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("no server certificate")
			}
			h := sha256.Sum256(rawCerts[0])
			fp := hex.EncodeToString(h[:])
			if expectedFingerprint != "" {
				if fp != expectedFingerprint {
					return fmt.Errorf("TLS fingerprint mismatch: got %s, expected %s", fp, expectedFingerprint)
				}
			} else {
				fmt.Fprintf(os.Stderr, "[*] WARNING: TLS insecure mode. Server fingerprint: %s\n", fp)
			}
			return nil
		},
	}
	return base, nil
}
