//go:build !windows

package pythonenv

import (
	"os"
	"path/filepath"
)

// GetPythonPath returns the path to the python executable.
// It tries to find a specific python environment on Linux (in /opt/ocr_env).
// Falls back to "python3".
func GetPythonPath() (string, error) {
	// Linux / Other
	ocrEnvPython := filepath.Join("/opt", "ocr_env", "bin", "python")
	if _, err := os.Stat(ocrEnvPython); err == nil {
		return ocrEnvPython, nil
	}

	return "python3", nil
}
