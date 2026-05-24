//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

func spawnUpdater(updaterExe, currentExe string) error {
	cmd := exec.Command(updaterExe, "--source", currentExe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		NoInheritHandles: true,
	}
	return cmd.Start()
}
