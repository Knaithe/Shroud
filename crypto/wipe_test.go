package crypto

import "testing"

func TestWipe(t *testing.T) {
	key := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03, 0x04}
	Wipe(key)
	for i, b := range key {
		if b != 0 {
			t.Fatalf("byte %d not zeroed: got 0x%02x", i, b)
		}
	}
}

func TestWipe_Nil(t *testing.T) {
	Wipe(nil) // must not panic
}

func TestWipe_Empty(t *testing.T) {
	Wipe([]byte{}) // must not panic
}
