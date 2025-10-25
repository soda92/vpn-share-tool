//go:build android

package gui

import (
	"fmt"

	"github.com/soda92/vpn-share-tool/native/android"
)

func getLanIPs() ([]string, error) {
	ip := android.GetIPAddress()
	if ip == "" {
		return nil, fmt.Errorf("could not get IP address from native call")
	}
	return []string{ip}, nil
}
