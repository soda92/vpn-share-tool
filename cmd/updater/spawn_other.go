//go:build !windows

package main

import (
	"os/exec"
)

func spawnProcessInNewConsole(exePath string, args ...string) error {
	cmd := exec.Command(exePath, args...)
	return cmd.Start()
}

func spawnProcessDetached(exePath string, args ...string) error {
	cmd := exec.Command(exePath, args...)
	return cmd.Start()
}
