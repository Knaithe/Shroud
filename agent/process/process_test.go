package process

import (
	"bytes"
	"testing"

	"Shroud/agent/initial"
	"Shroud/protocol"
)

func TestNewAgent(t *testing.T) {
	opts := &initial.Options{
		Secret:   []byte("test-secret"),
		Connect:  "127.0.0.1:8080",
		Upstream: "raw",
	}

	agent := NewAgent(opts)
	if agent == nil {
		t.Fatal("NewAgent returned nil")
	}

	if agent.UUID != protocol.TEMP_UUID {
		t.Fatalf("expected UUID=%q, got %q", protocol.TEMP_UUID, agent.UUID)
	}

	if agent.Memo != "" {
		t.Fatalf("expected empty Memo, got %q", agent.Memo)
	}

	if agent.options != opts {
		t.Fatal("options not stored correctly")
	}

	if agent.childrenMessChan == nil {
		t.Fatal("childrenMessChan is nil")
	}

	if agent.mgr != nil {
		t.Fatal("mgr should be nil before Run")
	}
}

func TestNewAgent_MultipleInstances(t *testing.T) {
	opts1 := &initial.Options{Secret: []byte("s1"), Connect: "1.1.1.1:80"}
	opts2 := &initial.Options{Secret: []byte("s2"), Listen: ":9090"}

	a1 := NewAgent(opts1)
	a2 := NewAgent(opts2)

	if a1 == a2 {
		t.Fatal("expected different instances")
	}
	if bytes.Equal(a1.options.Secret, a2.options.Secret) {
		t.Fatal("expected different options")
	}
}

func TestChangeRoute_SingleHop(t *testing.T) {
	header := &protocol.Header{
		Route:    "uuid-child-1",
		RouteLen: uint32(len("uuid-child-1")),
	}

	childUUID := changeRoute(header)
	if childUUID != "uuid-child-1" {
		t.Fatalf("expected childUUID='uuid-child-1', got %q", childUUID)
	}
	if header.Route != "" {
		t.Fatalf("expected empty route after single hop, got %q", header.Route)
	}
	if header.RouteLen != 0 {
		t.Fatalf("expected RouteLen=0, got %d", header.RouteLen)
	}
}

func TestChangeRoute_MultiHop(t *testing.T) {
	header := &protocol.Header{
		Route:    "uuid-1:uuid-2:uuid-3",
		RouteLen: uint32(len("uuid-1:uuid-2:uuid-3")),
	}

	childUUID := changeRoute(header)
	if childUUID != "uuid-1" {
		t.Fatalf("expected first UUID='uuid-1', got %q", childUUID)
	}
	if header.Route != "uuid-2:uuid-3" {
		t.Fatalf("expected remaining route='uuid-2:uuid-3', got %q", header.Route)
	}
	if header.RouteLen != uint32(len("uuid-2:uuid-3")) {
		t.Fatalf("expected RouteLen=%d, got %d", len("uuid-2:uuid-3"), header.RouteLen)
	}

	// Second hop
	childUUID = changeRoute(header)
	if childUUID != "uuid-2" {
		t.Fatalf("expected second UUID='uuid-2', got %q", childUUID)
	}
	if header.Route != "uuid-3" {
		t.Fatalf("expected remaining route='uuid-3', got %q", header.Route)
	}

	// Third hop (last)
	childUUID = changeRoute(header)
	if childUUID != "uuid-3" {
		t.Fatalf("expected third UUID='uuid-3', got %q", childUUID)
	}
	if header.Route != "" {
		t.Fatalf("expected empty route, got %q", header.Route)
	}
	if header.RouteLen != 0 {
		t.Fatalf("expected RouteLen=0, got %d", header.RouteLen)
	}
}
