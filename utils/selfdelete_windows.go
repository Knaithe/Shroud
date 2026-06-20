//go:build windows

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func SelfDeleteBinary() error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command("cmd", "/C",
		fmt.Sprintf("ping -n 3 127.0.0.1 >nul & del /f /q \"%s\"", path))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	return cmd.Start()
}
