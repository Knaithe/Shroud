//go:build darwin

package utils

import "syscall"

func DisableCoreDump() {
	_ = syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{Cur: 0, Max: 0})
	// PT_DENY_ATTACH (31) prevents debugger attachment on macOS.
	_, _, _ = syscall.Syscall(syscall.SYS_PTRACE, 31, 0, 0)
}
