package share

import (
	"net"
	"strings"
)

type TorProxy struct {
	PeerAddr     string
	TorSocksAddr string
}

func NewTorProxy(peerAddr, torSocksAddr string) *TorProxy {
	proxy := new(TorProxy)
	proxy.PeerAddr = peerAddr
	proxy.TorSocksAddr = torSocksAddr
	return proxy
}

func (proxy *TorProxy) Dial() (net.Conn, error) {
	socks := NewSocks5Proxy(proxy.PeerAddr, proxy.TorSocksAddr, "", "")
	return socks.Dial()
}

func IsOnionAddress(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return strings.HasSuffix(strings.ToLower(host), ".onion")
}
