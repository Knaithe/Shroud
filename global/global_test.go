package global

import (
	"bytes"
	"net"
	"testing"

	"Shroud/utils"
)

// helper to create a paired connection for testing.
func testConn(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	s, c := net.Pipe()
	t.Cleanup(func() { s.Close(); c.Close() })
	return s, c
}

func TestInitSession(t *testing.T) {
	conn, _ := testConn(t)
	cryptoKey := []byte("secret123")
	InitSession(conn, cryptoKey, "uuid-abc")

	if Session == nil {
		t.Fatal("Session is nil after InitSession")
	}
	if Session.Component == nil {
		t.Fatal("Component is nil")
	}
	if !bytes.Equal(Session.Component.CryptoKey, cryptoKey) {
		t.Fatalf("CryptoKey = %x, want %x", Session.Component.CryptoKey, cryptoKey)
	}
	if Session.Component.UUID != "uuid-abc" {
		t.Fatalf("UUID = %s, want uuid-abc", Session.Component.UUID)
	}
	if Session.Component.Conn != conn {
		t.Fatal("Conn not set correctly")
	}
}

func TestInitialGComponent(t *testing.T) {
	conn, _ := testConn(t)
	cryptoKey := []byte("sec")
	InitialGComponent(conn, cryptoKey, "uid")

	if G_Component == nil {
		t.Fatal("G_Component is nil")
	}
	if !bytes.Equal(G_Component.CryptoKey, cryptoKey) {
		t.Fatalf("CryptoKey = %x, want %x", G_Component.CryptoKey, cryptoKey)
	}
	if G_Component.UUID != "uid" {
		t.Fatalf("UUID = %s", G_Component.UUID)
	}
	if G_Component.Conn != conn {
		t.Fatal("Conn mismatch")
	}
	// G_Component should be the same pointer as Session.Component
	if G_Component != Session.Component {
		t.Fatal("G_Component is not Session.Component")
	}
}

func TestUpdateGComponent(t *testing.T) {
	conn1, _ := testConn(t)
	conn2, _ := testConn(t)

	InitialGComponent(conn1, []byte("s"), "u")
	UpdateGComponent(conn2)

	sc, ok := Session.Component.Conn.(*utils.SafeConn)
	if !ok || sc.Conn != conn2 {
		t.Fatal("Conn not updated")
	}
}

func TestUpdateConn(t *testing.T) {
	conn1, _ := testConn(t)
	conn2, _ := testConn(t)

	InitSession(conn1, []byte("s"), "u")
	Session.UpdateConn(conn2)

	sc, ok := Session.Component.Conn.(*utils.SafeConn)
	if !ok || sc.Conn != conn2 {
		t.Fatal("UpdateConn did not update conn")
	}
}

func TestSwapGComponentConn(t *testing.T) {
	conn1, _ := testConn(t)
	conn2, _ := testConn(t)

	InitialGComponent(conn1, []byte("s"), "u")
	old := SwapGComponentConn(conn2)

	if sc, ok := old.(*utils.SafeConn); ok {
		if sc.Conn != conn1 {
			t.Fatal("SwapGComponentConn did not return old conn")
		}
	}
	sc, ok := Session.Component.Conn.(*utils.SafeConn)
	if !ok || sc.Conn != conn2 {
		t.Fatal("SwapGComponentConn did not set new conn")
	}
}

func TestSwapConn(t *testing.T) {
	conn1, _ := testConn(t)
	conn2, _ := testConn(t)

	InitSession(conn1, []byte("s"), "u")
	old := Session.SwapConn(conn2)

	if sc, ok := old.(*utils.SafeConn); ok {
		if sc.Conn != conn1 {
			t.Fatal("SwapConn did not return old conn")
		}
	}
	sc, ok := Session.Component.Conn.(*utils.SafeConn)
	if !ok || sc.Conn != conn2 {
		t.Fatal("SwapConn did not set new conn")
	}
}

func TestSetGetTransportMode(t *testing.T) {
	conn, _ := testConn(t)
	InitSession(conn, []byte("s"), "u")

	// default mode is "raw"
	if m := Session.GetTransportMode(); m != "raw" {
		t.Fatalf("default mode = %s, want raw", m)
	}

	Session.SetTransportMode("ws")
	if m := Session.GetTransportMode(); m != "ws" {
		t.Fatalf("mode = %s, want ws", m)
	}

	// package-level accessors
	SetTransportMode("raw")
	if m := GetTransportMode(); m != "raw" {
		t.Fatalf("GetTransportMode() = %s, want raw", m)
	}
}

func TestSignalTransportSwitch(t *testing.T) {
	conn, _ := testConn(t)
	InitSession(conn, []byte("s"), "u")

	Session.SignalTransportSwitch()

	select {
	case <-Session.TransportSwitch:
		// received signal, ok
	default:
		t.Fatal("no signal received on TransportSwitch channel")
	}

	// package-level accessor
	SignalTransportSwitch()

	select {
	case <-Session.TransportSwitch:
		// ok
	default:
		t.Fatal("no signal from package-level SignalTransportSwitch")
	}
}
