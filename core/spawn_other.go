//go:build !windows

package core

import (
	"fmt"
)

func spawnUpdater(updaterExe, currentExe string) error {
	return fmt.Errorf("updater is only supported on Windows")
}
