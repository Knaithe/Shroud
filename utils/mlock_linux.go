//go:build linux

package utils

import "golang.org/x/sys/unix"

func MlockBytes(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return unix.Mlock(b)
}

func MunlockBytes(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return unix.Munlock(b)
}
