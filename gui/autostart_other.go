//go:build !windows

package gui

import "log"

func SetAutostart(enable bool) {
	if enable {
		log.Printf("Autostart not implemented on this OS.")
	}
}

func IsAutostartEnabled() bool {
	return false
}
