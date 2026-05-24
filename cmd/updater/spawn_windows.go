//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func spawnProcess(exePath string, args ...string) error {
	cmd := exec.Command(exePath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		NoInheritHandles: true,
	}
	return cmd.Start()
}
