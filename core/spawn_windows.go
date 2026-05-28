//go:build windows

package core

import (
	"os/exec"
	"syscall"
)

const CREATE_NEW_CONSOLE = 0x00000010

func spawnUpdater(updaterExe, currentExe string) error {
	cmd := exec.Command(updaterExe, "--source", currentExe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		NoInheritHandles: true,
		CreationFlags:    CREATE_NEW_CONSOLE,
	}
	return cmd.Start()
}
