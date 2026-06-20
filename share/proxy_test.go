package share

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
)

// --- Socks5Proxy tests ---

func TestNewSocks5Proxy(t *testing.T) {
	p := NewSocks5Proxy("10.0.0.1:8080", "127.0.0.1:1080", "alice", "secret")
	if p.PeerAddr != "10.0.0.1:8080" {
		t.Fatalf("PeerAddr: want %q, got %q", "10.0.0.1:8080", p.PeerAddr)
	}
	if p.ProxyAddr != "127.0.0.1:1080" {
		t.Fatalf("ProxyAddr: want %q, got %q", "127.0.0.1:1080", p.ProxyAddr)
	}
	if p.UserName != "alice" {
		t.Fatalf("UserName: want %q, got %q", "alice", p.UserName)
	}
	if p.Password != "secret" {
		t.Fatalf("Password: want %q, got %q", "secret", p.Password)
	}
}

func TestNewSocks5Proxy_NoAuth(t *testing.T) {
	p := NewSocks5Proxy("10.0.0.1:443", "127.0.0.1:1080", "", "")
	if p.UserName != "" || p.Password != "" {
		t.Fatal("expected empty credentials for no-auth proxy")
	}
}

// mockSocks5Server starts a listener that speaks the SOCKS5 handshake.
// authMode: 0x00 = no auth, 0x02 = username/password.
// rejectAuth: if true and authMode==0x02, respond with auth failure.
// connectReply: the REP byte in the connect response (0x00=success, else error).
func mockSocks5Server(t *testing.T, authMode byte, rejectAuth bool, connectReply byte) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// --- greeting ---
		greeting := make([]byte, 3)
		if _, err := io.ReadFull(conn, greeting); err != nil {
			return
		}

		// method selection
		conn.Write([]byte{0x05, authMode})

		// --- auth sub-negotiation ---
		if authMode == 0x02 {
			verBuf := make([]byte, 1)
			if _, err := io.ReadFull(conn, verBuf); err != nil {
				return
			}
			ulenBuf := make([]byte, 1)
			if _, err := io.ReadFull(conn, ulenBuf); err != nil {
				return
			}
			uname := make([]byte, ulenBuf[0])
			if _, err := io.ReadFull(conn, uname); err != nil {
				return
			}
			plenBuf := make([]byte, 1)
			if _, err := io.ReadFull(conn, plenBuf); err != nil {
				return
			}
			passwd := make([]byte, plenBuf[0])
			if _, err := io.ReadFull(conn, passwd); err != nil {
				return
			}

			if rejectAuth {
				conn.Write([]byte{0x01, 0x01}) // status 0x01 = failure
				return
			}
			conn.Write([]byte{0x01, 0x00}) // status 0x00 = success
		}

		// --- connect request ---
		reqHeader := make([]byte, 4)
		if _, err := io.ReadFull(conn, reqHeader); err != nil {
			return
		}
		atyp := reqHeader[3]
		switch atyp {
		case 0x01: // IPv4
			buf := make([]byte, 6) // 4 addr + 2 port
			io.ReadFull(conn, buf)
		case 0x03: // domain
			lenBuf := make([]byte, 1)
			io.ReadFull(conn, lenBuf)
			buf := make([]byte, int(lenBuf[0])+2)
			io.ReadFull(conn, buf)
		case 0x04: // IPv6
			buf := make([]byte, 18) // 16 addr + 2 port
			io.ReadFull(conn, buf)
		}

		// --- connect reply ---
		reply := make([]byte, 10)
		reply[0] = 0x05
		reply[1] = connectReply
		reply[2] = 0x00
		reply[3] = 0x01 // IPv4 bind addr
		// bind addr 0.0.0.0:0 (6 zero bytes) already zeroed
		conn.Write(reply)
	}()

	return ln.Addr().String(), func() { ln.Close() }
}

func TestSocks5Proxy_Dial_NoAuth(t *testing.T) {
	addr, cleanup := mockSocks5Server(t, 0x00, false, 0x00)
	defer cleanup()

	p := NewSocks5Proxy("192.168.1.1:80", addr, "", "")
	conn, err := p.Dial()
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	conn.Close()
}

func TestSocks5Proxy_Dial_UsernamePassword(t *testing.T) {
	addr, cleanup := mockSocks5Server(t, 0x02, false, 0x00)
	defer cleanup()

	p := NewSocks5Proxy("192.168.1.1:80", addr, "user", "pass")
	conn, err := p.Dial()
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	conn.Close()
}

func TestSocks5Proxy_Dial_AuthFailure(t *testing.T) {
	addr, cleanup := mockSocks5Server(t, 0x02, true, 0x00)
	defer cleanup()

	p := NewSocks5Proxy("192.168.1.1:80", addr, "user", "wrong")
	conn, err := p.Dial()
	if err == nil {
		conn.Close()
		t.Fatal("expected auth failure error, got nil")
	}
	if err.Error() != "wrong username/password" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSocks5Proxy_Dial_ConnectError(t *testing.T) {
	addr, cleanup := mockSocks5Server(t, 0x00, false, 0x05) // 0x05 = connection refused
	defer cleanup()

	p := NewSocks5Proxy("192.168.1.1:80", addr, "", "")
	conn, err := p.Dial()
	if err == nil {
		conn.Close()
		t.Fatal("expected server error, got nil")
	}
	if err.Error() != "proxy server error" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSocks5Proxy_Dial_DomainTarget(t *testing.T) {
	addr, cleanup := mockSocks5Server(t, 0x00, false, 0x00)
	defer cleanup()

	p := NewSocks5Proxy("example.com:443", addr, "", "")
	conn, err := p.Dial()
	if err != nil {
		t.Fatalf("Dial with domain target failed: %v", err)
	}
	conn.Close()
}

func TestSocks5Proxy_Dial_IPv6Target(t *testing.T) {
	// Build a mock that returns IPv6 bind address type in the reply.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// greeting
		greeting := make([]byte, 3)
		io.ReadFull(conn, greeting)
		conn.Write([]byte{0x05, 0x00})

		// connect request: read header
		reqHeader := make([]byte, 4)
		io.ReadFull(conn, reqHeader)
		// IPv6 addr type
		if reqHeader[3] == 0x04 {
			buf := make([]byte, 18) // 16 addr + 2 port
			io.ReadFull(conn, buf)
		}

		// reply with IPv4 bind
		reply := make([]byte, 10)
		reply[0] = 0x05
		reply[1] = 0x00
		reply[3] = 0x01
		conn.Write(reply)
	}()

	p := NewSocks5Proxy("[::1]:80", ln.Addr().String(), "", "")
	conn, err := p.Dial()
	if err != nil {
		t.Fatalf("Dial with IPv6 target failed: %v", err)
	}
	conn.Close()
}

func TestSocks5Proxy_Dial_ConnectionRefused(t *testing.T) {
	// Use a port that nothing is listening on.
	p := NewSocks5Proxy("192.168.1.1:80", "127.0.0.1:1", "", "")
	conn, err := p.Dial()
	if err == nil {
		conn.Close()
		t.Fatal("expected connection error, got nil")
	}
}

func TestSocks5Proxy_Dial_WrongAuthMethod(t *testing.T) {
	// Server responds with 0xFF (no acceptable methods).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		greeting := make([]byte, 3)
		io.ReadFull(conn, greeting)
		conn.Write([]byte{0x05, 0xFF})
	}()

	p := NewSocks5Proxy("192.168.1.1:80", ln.Addr().String(), "", "")
	conn, err := p.Dial()
	if err == nil {
		conn.Close()
		t.Fatal("expected wrong auth method error, got nil")
	}
	if err.Error() != "wrong auth method" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- HTTPProxy tests ---

func TestNewHTTPProxy(t *testing.T) {
	p := NewHTTPProxy("10.0.0.1:443", "127.0.0.1:8080")
	if p.PeerAddr != "10.0.0.1:443" {
		t.Fatalf("PeerAddr: want %q, got %q", "10.0.0.1:443", p.PeerAddr)
	}
	if p.ProxyAddr != "127.0.0.1:8080" {
		t.Fatalf("ProxyAddr: want %q, got %q", "127.0.0.1:8080", p.ProxyAddr)
	}
}

// mockHTTPProxy starts a mock HTTP proxy that reads the CONNECT request
// and responds with the given status line.
func mockHTTPProxy(t *testing.T, statusLine string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read request until \r\n\r\n
		buf := make([]byte, 4096)
		total := 0
		for {
			n, err := conn.Read(buf[total:])
			if err != nil {
				return
			}
			total += n
			if total >= 4 && string(buf[total-4:total]) == "\r\n\r\n" {
				break
			}
		}

		response := statusLine + "\r\n\r\n"
		conn.Write([]byte(response))
	}()

	return ln.Addr().String(), func() { ln.Close() }
}

func TestHTTPProxy_Dial_Success(t *testing.T) {
	addr, cleanup := mockHTTPProxy(t, "HTTP/1.1 200 Connection Established")
	defer cleanup()

	p := NewHTTPProxy("10.0.0.1:443", addr)
	conn, err := p.Dial()
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	conn.Close()
}

func TestHTTPProxy_Dial_Forbidden(t *testing.T) {
	addr, cleanup := mockHTTPProxy(t, "HTTP/1.1 403 Forbidden")
	defer cleanup()

	p := NewHTTPProxy("10.0.0.1:443", addr)
	conn, err := p.Dial()
	if err == nil {
		conn.Close()
		t.Fatal("expected error for 403 response, got nil")
	}
	if err.Error() != "proxy server error" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPProxy_Dial_ConnectionRefused(t *testing.T) {
	p := NewHTTPProxy("10.0.0.1:443", "127.0.0.1:1")
	conn, err := p.Dial()
	if err == nil {
		conn.Close()
		t.Fatal("expected error for connection refused, got nil")
	}
	if err.Error() != "proxy server error" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPProxy_Dial_VerifiesConnectPayload(t *testing.T) {
	// Verify the proxy actually sends a properly formatted CONNECT request.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	done := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- ""
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		total := 0
		for {
			n, err := conn.Read(buf[total:])
			if err != nil {
				done <- ""
				return
			}
			total += n
			if total >= 4 && string(buf[total-4:total]) == "\r\n\r\n" {
				break
			}
		}
		done <- string(buf[:total])

		conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	}()

	target := "example.com:443"
	p := NewHTTPProxy(target, ln.Addr().String())
	conn, err := p.Dial()
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	conn.Close()

	request := <-done
	expected := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nContent-Length: 0\r\n\r\n", target)
	if request != expected {
		t.Fatalf("CONNECT payload mismatch:\nwant: %q\ngot:  %q", expected, request)
	}
}

// --- Proxy interface tests ---

func TestSocks5ProxyImplementsProxy(t *testing.T) {
	var _ Proxy = (*Socks5Proxy)(nil)
}

func TestHTTPProxyImplementsProxy(t *testing.T) {
	var _ Proxy = (*HTTPProxy)(nil)
}

// --- Edge case: SOCKS5 domain reply type ---

func TestSocks5Proxy_Dial_DomainReplyType(t *testing.T) {
	// Server replies with atyp=0x03 (domain) in the connect response.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		greeting := make([]byte, 3)
		io.ReadFull(conn, greeting)
		conn.Write([]byte{0x05, 0x00})

		// read connect request header
		reqHeader := make([]byte, 4)
		io.ReadFull(conn, reqHeader)

		// consume rest of connect request based on atyp
		switch reqHeader[3] {
		case 0x01:
			buf := make([]byte, 6)
			io.ReadFull(conn, buf)
		case 0x03:
			lenBuf := make([]byte, 1)
			io.ReadFull(conn, lenBuf)
			buf := make([]byte, int(lenBuf[0])+2)
			io.ReadFull(conn, buf)
		case 0x04:
			buf := make([]byte, 18)
			io.ReadFull(conn, buf)
		}

		// reply with domain bind address
		domainBind := "a.com"
		reply := make([]byte, 0, 7+len(domainBind))
		reply = append(reply, 0x05, 0x00, 0x00, 0x03)
		reply = append(reply, byte(len(domainBind)))
		reply = append(reply, []byte(domainBind)...)
		port := make([]byte, 2)
		binary.BigEndian.PutUint16(port, 1080)
		reply = append(reply, port...)
		conn.Write(reply)
	}()

	p := NewSocks5Proxy("192.168.1.1:80", ln.Addr().String(), "", "")
	conn, err := p.Dial()
	if err != nil {
		t.Fatalf("Dial with domain reply type failed: %v", err)
	}
	conn.Close()
}
