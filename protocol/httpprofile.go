package protocol

import (
	"crypto/rand"
	"encoding/binary"
	"sync"
)

var (
	userAgents   []string
	uaMu         sync.Mutex
	frontDomain  string
	customOrigin string
)

func SetUserAgents(agents []string) {
	uaMu.Lock()
	userAgents = append([]string(nil), agents...)
	uaMu.Unlock()
}

func SetFrontDomain(domain string) { frontDomain = domain }

func SetOrigin(origin string) { customOrigin = origin }

func RotateUserAgent() string {
	uaMu.Lock()
	defer uaMu.Unlock()
	if len(userAgents) == 0 {
		return ""
	}
	var b [8]byte
	rand.Read(b[:])
	idx := binary.BigEndian.Uint64(b[:]) % uint64(len(userAgents))
	return userAgents[idx]
}

func EffectiveHost(actualDomain string) string {
	if frontDomain != "" {
		return frontDomain
	}
	return actualDomain
}

func EffectiveOrigin() string {
	if customOrigin != "" {
		return customOrigin
	}
	return "https://www.google.com"
}
