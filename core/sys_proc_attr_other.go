//go:build !windows

package core

import "syscall"

func getSysProcAttr() *syscall.SysProcAttr {
	return nil
}
