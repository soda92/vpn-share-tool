//go:build !windows

package main

import (
	"os/exec"
)

func spawnProcess(exePath string, args ...string) error {
	cmd := exec.Command(exePath, args...)
	return cmd.Start()
}
