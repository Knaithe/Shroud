package share

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// NewTorControl
// ---------------------------------------------------------------------------

func TestNewTorControl_Fields(t *testing.T) {
	tc := NewTorControl("127.0.0.1:9051", "my-pass")
	if tc.addr != "127.0.0.1:9051" {
		t.Fatalf("addr = %q, want %q", tc.addr, "127.0.0.1:9051")
	}
	if tc.password != "my-pass" {
		t.Fatalf("password = %q, want %q", tc.password, "my-pass")
	}
	if tc.conn != nil {
		t.Fatal("conn should be nil before Connect()")
	}
	if tc.reader != nil {
		t.Fatal("reader should be nil before Connect()")
	}
}

func TestNewTorControl_EmptyPassword(t *testing.T) {
	tc := NewTorControl("127.0.0.1:9051", "")
	if tc.password != "" {
		t.Fatalf("password = %q, want empty", tc.password)
	}
}

// ---------------------------------------------------------------------------
// Connect
// ---------------------------------------------------------------------------

func TestConnect_Success(t *testing.T) {
	// Start a mock TCP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	defer ln.Close()

	// Accept one connection in the background
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()

	tc := NewTorControl(ln.Addr().String(), "")
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer tc.Close()

	if tc.conn == nil {
		t.Fatal("conn should be non-nil after Connect()")
	}
	if tc.reader == nil {
		t.Fatal("reader should be non-nil after Connect()")
	}

	// Clean up server-side connection
	serverConn := <-accepted
	serverConn.Close()
}

func TestConnect_Failure(t *testing.T) {
	// Use an address where nothing is listening
	tc := NewTorControl("127.0.0.1:1", "")
	err := tc.Connect()
	if err == nil {
		t.Fatal("expected Connect() to fail on unreachable port")
	}
	if !strings.Contains(err.Error(), "cannot connect to Tor control port") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// sendCommand
// ---------------------------------------------------------------------------

func TestSendCommand_WritesAndReads(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		// Read the command sent by the client
		line, _ := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		// Respond with 250 OK
		fmt.Fprintf(conn, "250 OK\r\n")
		_ = line
	}()

	tc := NewTorControl(ln.Addr().String(), "")
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer tc.Close()

	resp, err := tc.sendCommand("GETINFO version\r\n")
	if err != nil {
		t.Fatalf("sendCommand error: %v", err)
	}
	if !strings.HasPrefix(resp, "250") {
		t.Fatalf("expected 250 response, got: %q", resp)
	}
}

func TestSendCommand_NotConnected(t *testing.T) {
	tc := NewTorControl("127.0.0.1:9051", "")
	// Don't call Connect()
	_, err := tc.sendCommand("AUTHENTICATE\r\n")
	if err == nil {
		t.Fatal("expected error when not connected")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Authenticate
// ---------------------------------------------------------------------------

func TestAuthenticate_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		line, _ := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		// Verify the client sent AUTHENTICATE with password
		if !strings.HasPrefix(line, "AUTHENTICATE") {
			fmt.Fprintf(conn, "510 Unrecognized command\r\n")
			return
		}
		fmt.Fprintf(conn, "250 OK\r\n")
	}()

	tc := NewTorControl(ln.Addr().String(), "test-password")
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer tc.Close()

	if err := tc.Authenticate(); err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}
}

func TestAuthenticate_NoPassword(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		line, _ := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line != "AUTHENTICATE" {
			fmt.Fprintf(conn, "510 Bad command\r\n")
			return
		}
		fmt.Fprintf(conn, "250 OK\r\n")
	}()

	tc := NewTorControl(ln.Addr().String(), "")
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer tc.Close()

	if err := tc.Authenticate(); err != nil {
		t.Fatalf("Authenticate() without password error: %v", err)
	}
}

func TestAuthenticate_Failure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		reader.ReadString('\n')
		fmt.Fprintf(conn, "515 Bad authentication\r\n")
	}()

	tc := NewTorControl(ln.Addr().String(), "wrong-password")
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer tc.Close()

	err = tc.Authenticate()
	if err == nil {
		t.Fatal("expected authentication failure")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestClose_SetsConnNil(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Read and discard the QUIT command
		reader := bufio.NewReader(conn)
		reader.ReadString('\n')
		fmt.Fprintf(conn, "250 closing connection\r\n")
	}()

	tc := NewTorControl(ln.Addr().String(), "")
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	tc.Close()

	if tc.conn != nil {
		t.Fatal("conn should be nil after Close()")
	}
}

func TestClose_DoubleClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		reader.ReadString('\n')
		fmt.Fprintf(conn, "250 closing connection\r\n")
	}()

	tc := NewTorControl(ln.Addr().String(), "")
	if err := tc.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	// Should not panic
	tc.Close()
	tc.Close()
}

func TestClose_WithoutConnect(t *testing.T) {
	tc := NewTorControl("127.0.0.1:9051", "")
	// Should not panic
	tc.Close()
}
