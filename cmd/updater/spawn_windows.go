//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

const CREATE_NEW_CONSOLE = 0x00000010

func spawnProcessInNewConsole(exePath string, args ...string) error {
	cmd := exec.Command(exePath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		NoInheritHandles: true,
		CreationFlags:    CREATE_NEW_CONSOLE,
	}
	return cmd.Start()
}

func spawnProcessDetached(exePath string, args ...string) error {
	cmd := exec.Command(exePath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		NoInheritHandles: true,
	}
	return cmd.Start()
}
