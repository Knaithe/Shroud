//go:build linux

package utils

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

func DisableCoreDump() {
	_ = unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
}

func MaskProcessName(name string) {
	nameBytes := make([]byte, 16)
	copy(nameBytes, name)
	_ = unix.Prctl(unix.PR_SET_NAME, uintptr(unsafe.Pointer(&nameBytes[0])), 0, 0, 0)
	os.Args = []string{name}
}
