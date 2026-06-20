//go:build linux

package utils

import "golang.org/x/sys/unix"

func DisableCoreDump() {
	_ = unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
}
