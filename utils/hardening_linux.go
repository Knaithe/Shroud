//go:build linux

package utils

import (
	"os"
	"syscall"
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

func ScrubCmdline() {
	_, _, e1 := syscall.Syscall(syscall.SYS_PRCTL, 36, 1, 0)
	if e1 != 0 {
		return
	}
	var z byte
	zptr := uintptr(unsafe.Pointer(&z))
	syscall.Syscall6(syscall.SYS_PRCTL, 35, 8, zptr, 0, 0, 0)
	syscall.Syscall6(syscall.SYS_PRCTL, 35, 9, zptr, 0, 0, 0)
}
