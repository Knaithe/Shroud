//go:build !windows

package utils

import "os"

func SelfDeleteBinary() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	return SecureRemoveFile(path)
}
