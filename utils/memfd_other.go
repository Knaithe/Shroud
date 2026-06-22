//go:build !linux

package utils

import (
	"errors"
	"os"
)

func MemfdCreate(name string) (*os.File, error) {
	return nil, errors.New("memfd_create is only supported on Linux")
}

func FilelessHarden() {}
