//go:build linux

package utils

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func MemfdCreate(name string) (*os.File, error) {
	fd, err := unix.MemfdCreate(name, unix.MFD_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("memfd_create: %w", err)
	}
	return os.NewFile(uintptr(fd), fmt.Sprintf("/proc/self/fd/%d", fd)), nil
}

func FilelessHarden() {
	_ = unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0)
}
