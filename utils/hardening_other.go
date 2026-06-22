//go:build !linux && !windows && !darwin

package utils

import "os"

func DisableCoreDump() {}

func MaskProcessName(name string) {
	os.Args = []string{name}
}
