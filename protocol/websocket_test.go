package protocol

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestGenerateNonce(t *testing.T) {
	t.Run("returns 24 bytes without error", func(t *testing.T) {
		nonce, err := generateNonce()
		if err != nil {
			t.Fatalf("generateNonce() returned error: %v", err)
		}
		if len(nonce) != 24 {
			t.Fatalf("expected 24 bytes, got %d", len(nonce))
		}
	})

	t.Run("two calls produce different nonces", func(t *testing.T) {
		n1, err1 := generateNonce()
		n2, err2 := generateNonce()
		if err1 != nil || err2 != nil {
			t.Fatalf("generateNonce() errors: %v, %v", err1, err2)
		}
		if string(n1) == string(n2) {
			t.Fatal("two consecutive nonces should differ")
		}
	})
}

func TestGetNonceAccept(t *testing.T) {
	t.Run("RFC 6455 test vector", func(t *testing.T) {
		// RFC 6455 Section 4.2.2 example:
		// Key: "dGhlIHNhbXBsZSBub25jZQ=="
		// Expected Accept: "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
		key := []byte("dGhlIHNhbXBsZSBub25jZQ==")
		accept, err := getNonceAccept(key)
		if err != nil {
			t.Fatalf("getNonceAccept() error: %v", err)
		}
		expected := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
		if string(accept) != expected {
			t.Fatalf("expected %q, got %q", expected, string(accept))
		}
	})

	t.Run("different keys produce different accepts", func(t *testing.T) {
		a1, _ := getNonceAccept([]byte("AAAAAAAAAAAAAAAAAAAAAA=="))
		a2, _ := getNonceAccept([]byte("BBBBBBBBBBBBBBBBBBBBBB=="))
		if string(a1) == string(a2) {
			t.Fatal("different keys should produce different accepts")
		}
	})
}

func TestContainsHeaderValue(t *testing.T) {
	resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=\r\n\r\n"

	t.Run("case insensitive header name, exact value match", func(t *testing.T) {
		if !containsHeaderValue(resp, "Upgrade", "websocket") {
			t.Fatal("should find Upgrade: websocket")
		}
		if !containsHeaderValue(resp, "upgrade", "websocket") {
			t.Fatal("header name match should be case insensitive")
		}
		if !containsHeaderValue(resp, "UPGRADE", "websocket") {
			t.Fatal("header name match should be case insensitive (uppercase)")
		}
	})

	t.Run("value is case sensitive", func(t *testing.T) {
		if containsHeaderValue(resp, "Upgrade", "WebSocket") {
			t.Fatal("value match should be case sensitive")
		}
	})

	t.Run("missing header returns false", func(t *testing.T) {
		if containsHeaderValue(resp, "X-Missing", "anything") {
			t.Fatal("missing header should return false")
		}
	})

	t.Run("wrong value returns false", func(t *testing.T) {
		if containsHeaderValue(resp, "Upgrade", "not-websocket") {
			t.Fatal("wrong value should return false")
		}
	})

	t.Run("accept header matched", func(t *testing.T) {
		if !containsHeaderValue(resp, "Sec-WebSocket-Accept", "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=") {
			t.Fatal("should find Sec-WebSocket-Accept value")
		}
	})
}

func TestWSProtoCNegotiate(t *testing.T) {
	oldPath := WebSocketPath()
	t.Cleanup(func() { _ = SetWebSocketPath(oldPath) })
	if err := SetWebSocketPath("/ws-test"); err != nil {
		t.Fatalf("SetWebSocketPath: %v", err)
	}
	t.Run("success with valid upgrade response", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		proto := &WSProto{
			domain: "example.com",
			conn:   clientConn,
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- proto.CNegotiate()
		}()

		// Server side: read the request and extract the key
		buf := make([]byte, 4096)
		var req string
		for {
			n, err := serverConn.Read(buf)
			if err != nil {
				t.Errorf("server read error: %v", err)
				return
			}
			req += string(buf[:n])
			if strings.Contains(req, "\r\n\r\n") {
				break
			}
		}

		// Verify request contains required fields
		if !strings.Contains(req, "GET /ws-test HTTP/1.1") {
			t.Errorf("request missing GET path, got:\n%s", req)
		}
		if !strings.Contains(req, "Host: example.com") {
			t.Errorf("request missing Host header, got:\n%s", req)
		}
		if !strings.Contains(req, "Upgrade: websocket") {
			t.Errorf("request missing Upgrade header")
		}

		// Extract key from request
		var key string
		for _, line := range strings.Split(req, "\r\n") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "Sec-WebSocket-Key" {
				key = strings.TrimSpace(parts[1])
				break
			}
		}
		if key == "" {
			t.Fatal("could not extract Sec-WebSocket-Key from request")
		}

		accept, err := getNonceAccept([]byte(key))
		if err != nil {
			t.Fatalf("getNonceAccept error: %v", err)
		}

		resp := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", string(accept))
		serverConn.Write([]byte(resp))

		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("CNegotiate() returned error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("CNegotiate() timed out")
		}
	})

	t.Run("failure with bad response", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		proto := &WSProto{
			domain: "example.com",
			conn:   clientConn,
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- proto.CNegotiate()
		}()

		// Read the request from the client
		buf := make([]byte, 4096)
		var req string
		for {
			n, err := serverConn.Read(buf)
			if err != nil {
				break
			}
			req += string(buf[:n])
			if strings.Contains(req, "\r\n\r\n") {
				break
			}
		}

		// Send a non-upgrade HTTP response
		resp := "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n"
		serverConn.Write([]byte(resp))

		select {
		case err := <-errCh:
			if err == nil {
				t.Fatal("CNegotiate() should have returned an error for non-upgrade response")
			}
			if !strings.Contains(err.Error(), "not websocket protocol") {
				t.Fatalf("expected 'not websocket protocol' error, got: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("CNegotiate() timed out")
		}
	})

	t.Run("failure with wrong accept value", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		proto := &WSProto{
			domain: "example.com",
			conn:   clientConn,
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- proto.CNegotiate()
		}()

		buf := make([]byte, 4096)
		var req string
		for {
			n, err := serverConn.Read(buf)
			if err != nil {
				break
			}
			req += string(buf[:n])
			if strings.Contains(req, "\r\n\r\n") {
				break
			}
		}

		// Send upgrade response with wrong accept value
		resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: WRONGVALUE0000000000000000000\r\n\r\n"
		serverConn.Write([]byte(resp))

		select {
		case err := <-errCh:
			if err == nil {
				t.Fatal("CNegotiate() should have returned an error for wrong accept")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("CNegotiate() timed out")
		}
	})
}

func TestWSProtoSNegotiate(t *testing.T) {
	oldPath := WebSocketPath()
	t.Cleanup(func() { _ = SetWebSocketPath(oldPath) })
	if err := SetWebSocketPath("/ws-test"); err != nil {
		t.Fatalf("SetWebSocketPath: %v", err)
	}
	t.Run("success with valid upgrade request", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		proto := &WSProto{
			domain: "example.com",
			conn:   serverConn,
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- proto.SNegotiate()
		}()

		// Client side: send a valid WS upgrade request with a known key
		key := "dGhlIHNhbXBsZSBub25jZQ=="
		req := fmt.Sprintf("GET /ws-test HTTP/1.1\r\nHost: example.com\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", key)
		clientConn.Write([]byte(req))

		// Read the server's response
		buf := make([]byte, 4096)
		var resp string
		for {
			n, err := clientConn.Read(buf)
			if err != nil {
				break
			}
			resp += string(buf[:n])
			if strings.Contains(resp, "\r\n\r\n") {
				break
			}
		}

		// Wait for SNegotiate to complete
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("SNegotiate() returned error: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("SNegotiate() timed out")
		}

		// Verify response
		if !strings.Contains(resp, "101 Switching Protocols") {
			t.Fatalf("response missing 101 status, got:\n%s", resp)
		}
		if !strings.Contains(resp, "Upgrade: websocket") {
			t.Fatalf("response missing Upgrade header, got:\n%s", resp)
		}

		// Verify the accept value matches RFC 6455 known result
		expectedAccept := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="
		if !strings.Contains(resp, "Sec-WebSocket-Accept: "+expectedAccept) {
			t.Fatalf("response missing correct accept value, got:\n%s", resp)
		}
	})

	t.Run("failure with missing key", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		proto := &WSProto{
			domain: "example.com",
			conn:   serverConn,
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- proto.SNegotiate()
		}()

		// Send request without Sec-WebSocket-Key
		req := "GET /ws-test HTTP/1.1\r\nHost: example.com\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"
		clientConn.Write([]byte(req))

		select {
		case err := <-errCh:
			if err == nil {
				t.Fatal("SNegotiate() should have returned an error for missing key")
			}
			if !strings.Contains(err.Error(), "Sec-WebSocket-Key") {
				t.Fatalf("expected error about Sec-WebSocket-Key, got: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("SNegotiate() timed out")
		}
	})

	t.Run("failure with bad request", func(t *testing.T) {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()

		proto := &WSProto{
			domain: "example.com",
			conn:   serverConn,
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- proto.SNegotiate()
		}()

		// Send garbage followed by proper termination
		req := "NOT A VALID HTTP REQUEST\r\n\r\n"
		clientConn.Write([]byte(req))

		select {
		case err := <-errCh:
			if err == nil {
				t.Fatal("SNegotiate() should have returned an error for bad request")
			}
		case <-time.After(5 * time.Second):
			t.Fatal("SNegotiate() timed out")
		}
	})
}
