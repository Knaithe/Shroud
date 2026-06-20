//go:build !linux && !windows && !darwin

package utils

func DisableCoreDump() {}
