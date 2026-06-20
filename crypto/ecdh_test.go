package crypto

import (
	"bytes"
	"net"
	"testing"
)

func TestECDHExchangeProducesSameKey(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	salt := []byte("test-salt-for-binding")
	var clientKey, serverKey []byte
	var clientErr, serverErr error

	done := make(chan struct{})
	go func() {
		clientKey, clientErr = ECDHExchangeActive(client, salt)
		close(done)
	}()
	serverKey, serverErr = ECDHExchangePassive(server, salt)
	<-done

	if clientErr != nil {
		t.Fatalf("client ECDH failed: %v", clientErr)
	}
	if serverErr != nil {
		t.Fatalf("server ECDH failed: %v", serverErr)
	}
	if !bytes.Equal(clientKey, serverKey) {
		t.Fatalf("keys don't match: client=%x server=%x", clientKey, serverKey)
	}
	if len(clientKey) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(clientKey))
	}
}

func TestECDHDifferentSaltProducesDifferentKey(t *testing.T) {
	c1, s1 := net.Pipe()
	c2, s2 := net.Pipe()
	defer c1.Close()
	defer s1.Close()
	defer c2.Close()
	defer s2.Close()

	var key1, key2 []byte
	done := make(chan struct{}, 2)

	go func() {
		key1, _ = ECDHExchangeActive(c1, []byte("salt-A"))
		done <- struct{}{}
	}()
	go func() {
		ECDHExchangePassive(s1, []byte("salt-A"))
		done <- struct{}{}
	}()
	<-done
	<-done

	go func() {
		key2, _ = ECDHExchangeActive(c2, []byte("salt-B"))
		done <- struct{}{}
	}()
	go func() {
		ECDHExchangePassive(s2, []byte("salt-B"))
		done <- struct{}{}
	}()
	<-done
	<-done

	if bytes.Equal(key1, key2) {
		t.Fatal("different salts should produce different keys")
	}
}

func TestECDHEachSessionProducesUniqueKey(t *testing.T) {
	salt := []byte("same-salt")
	keys := make([][]byte, 3)

	for i := range keys {
		c, s := net.Pipe()
		done := make(chan struct{})
		go func() {
			keys[i], _ = ECDHExchangeActive(c, salt)
			close(done)
		}()
		ECDHExchangePassive(s, salt)
		<-done
		c.Close()
		s.Close()
	}

	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if bytes.Equal(keys[i], keys[j]) {
				t.Fatalf("sessions %d and %d produced same key", i, j)
			}
		}
	}
}

func TestECDHNilSalt(t *testing.T) {
	c, s := net.Pipe()
	defer c.Close()
	defer s.Close()

	var ck, sk []byte
	done := make(chan struct{})
	go func() {
		ck, _ = ECDHExchangeActive(c, nil)
		close(done)
	}()
	sk, _ = ECDHExchangePassive(s, nil)
	<-done

	if !bytes.Equal(ck, sk) {
		t.Fatal("nil salt: keys don't match")
	}
}
