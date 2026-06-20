//go:build windows

package utils

import "golang.org/x/sys/windows"

func DisableCoreDump() {
	const (
		semFailCriticalErrors = 0x0001
		semNoGPFaultErrorBox  = 0x0002
		semNoOpenFileErrorBox = 0x8000
	)
	windows.SetErrorMode(semFailCriticalErrors | semNoGPFaultErrorBox | semNoOpenFileErrorBox)
}
