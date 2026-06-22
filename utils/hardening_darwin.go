//go:build darwin

package utils

import (
	"os"
	"syscall"
)

func DisableCoreDump() {
	_ = syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{Cur: 0, Max: 0})
	_, _, _ = syscall.Syscall(syscall.SYS_PTRACE, 31, 0, 0)
}

func MaskProcessName(name string) {
	os.Args = []string{name}
}
