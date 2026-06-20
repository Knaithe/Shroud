//go:build !linux && !windows && !darwin

package utils

func MlockBytes(b []byte) error   { return nil }
func MunlockBytes(b []byte) error { return nil }
