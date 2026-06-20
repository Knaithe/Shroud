package share

import (
	"testing"
)

func TestIsOnionAddress(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{"abc.onion:80", true},
		{"abc.onion", true},
		{"ABC.ONION:443", true},
		{"example.com:80", false},
		{"192.168.1.1:22", false},
		{"notanonion.com", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsOnionAddress(tt.addr)
		if got != tt.want {
			t.Errorf("IsOnionAddress(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestNewTorProxy(t *testing.T) {
	proxy := NewTorProxy("target.onion:80", "127.0.0.1:9050")

	if proxy == nil {
		t.Fatal("NewTorProxy returned nil")
	}
	if proxy.PeerAddr != "target.onion:80" {
		t.Fatalf("PeerAddr = %s, want target.onion:80", proxy.PeerAddr)
	}
	if proxy.TorSocksAddr != "127.0.0.1:9050" {
		t.Fatalf("TorSocksAddr = %s, want 127.0.0.1:9050", proxy.TorSocksAddr)
	}
}
