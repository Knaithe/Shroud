//go:build darwin

package utils

import "syscall"

func MlockBytes(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return syscall.Mlock(b)
}

func MunlockBytes(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return syscall.Munlock(b)
}
