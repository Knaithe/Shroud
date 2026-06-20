package process

import (
	"crypto/rand"
	"encoding/binary"
	"math"
	"time"
)

const maxBackoff = 300 * time.Second

func backoffDuration(attempt int, base time.Duration) time.Duration {
	d := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	if d > maxBackoff {
		d = maxBackoff
	}
	jitter := time.Duration(cryptoRandFloat() * 0.3 * float64(d))
	return d + jitter
}

func cryptoRandFloat() float64 {
	var b [8]byte
	rand.Read(b[:])
	return float64(binary.BigEndian.Uint64(b[:])) / float64(math.MaxUint64)
}
