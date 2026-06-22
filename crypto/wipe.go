package crypto

import "runtime"

// Wipe zeros a byte slice in-place to reduce key exposure in memory.
func Wipe(b []byte) {
	for i := range b {
		b[i] = 0
	}
	runtime.KeepAlive(b)
}
