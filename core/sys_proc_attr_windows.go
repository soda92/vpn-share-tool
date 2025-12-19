//go:build windows

package core

import "syscall"
// still don't work
func getSysProcAttr() *syscall.SysProcAttr {
	const CREATE_NO_WINDOW = 0x08000000
	return &syscall.SysProcAttr{
		CreationFlags: CREATE_NO_WINDOW,
	}
}
