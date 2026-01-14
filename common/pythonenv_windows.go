package common

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// GetPythonPath returns the path to the python executable.
// It tries to find a specific python environment on Windows (via registry).
// Falls back to "python".
func GetPythonPath() (string, error) {
	// Try to find "Digital Worker RPA Platform" python from registry
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\数字员工平台`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()

		val, _, err := k.GetStringValue("PythonPath")
		if err == nil && val != "" {
			exePath := filepath.Join(val, "python.exe")
			if _, err := os.Stat(exePath); err == nil {
				return exePath, nil
			}
		}
	}
	// Fallback
	return "python", nil
}
