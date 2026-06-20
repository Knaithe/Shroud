package crypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"net"

	"golang.org/x/crypto/hkdf"
)

const linkKeyInfo = "shroud-link-v1"

func ecdhExchange(conn net.Conn, sendFirst bool) ([]byte, error) {
	curve := ecdh.X25519()
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ECDH key: %w", err)
	}

	pubBytes := privKey.PublicKey().Bytes()
	peerPub := make([]byte, 32)

	if sendFirst {
		if err := writeFull(conn, pubBytes); err != nil {
			return nil, fmt.Errorf("send ECDH public key: %w", err)
		}
		if _, err := io.ReadFull(conn, peerPub); err != nil {
			return nil, fmt.Errorf("read ECDH public key: %w", err)
		}
	} else {
		if _, err := io.ReadFull(conn, peerPub); err != nil {
			return nil, fmt.Errorf("read ECDH public key: %w", err)
		}
		if err := writeFull(conn, pubBytes); err != nil {
			return nil, fmt.Errorf("send ECDH public key: %w", err)
		}
	}

	peerKey, err := curve.NewPublicKey(peerPub)
	if err != nil {
		return nil, fmt.Errorf("parse peer public key: %w", err)
	}

	shared, err := privKey.ECDH(peerKey)
	if err != nil {
		return nil, fmt.Errorf("ECDH compute: %w", err)
	}

	return deriveLinkKey(shared, nil)
}

func ECDHExchangeActive(conn net.Conn, salt []byte) ([]byte, error) {
	return ecdhExchangeWithSalt(conn, true, salt)
}

func ECDHExchangePassive(conn net.Conn, salt []byte) ([]byte, error) {
	return ecdhExchangeWithSalt(conn, false, salt)
}

func ecdhExchangeWithSalt(conn net.Conn, sendFirst bool, salt []byte) ([]byte, error) {
	curve := ecdh.X25519()
	privKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ECDH key: %w", err)
	}

	pubBytes := privKey.PublicKey().Bytes()
	peerPub := make([]byte, 32)

	if sendFirst {
		if err := writeFull(conn, pubBytes); err != nil {
			return nil, fmt.Errorf("send ECDH public key: %w", err)
		}
		if _, err := io.ReadFull(conn, peerPub); err != nil {
			return nil, fmt.Errorf("read ECDH public key: %w", err)
		}
	} else {
		if _, err := io.ReadFull(conn, peerPub); err != nil {
			return nil, fmt.Errorf("read ECDH public key: %w", err)
		}
		if err := writeFull(conn, pubBytes); err != nil {
			return nil, fmt.Errorf("send ECDH public key: %w", err)
		}
	}

	peerKey, err := curve.NewPublicKey(peerPub)
	if err != nil {
		return nil, fmt.Errorf("parse peer public key: %w", err)
	}

	shared, err := privKey.ECDH(peerKey)
	if err != nil {
		return nil, fmt.Errorf("ECDH compute: %w", err)
	}

	return deriveLinkKey(shared, salt)
}

func deriveLinkKey(shared, salt []byte) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, shared, salt, []byte(linkKeyInfo))
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("HKDF derive: %w", err)
	}
	return key, nil
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
