package pythonenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// GetPythonPath returns the path to the python executable.
// It tries to find a specific python environment on Windows (via registry)
// and on Linux (in /opt/ocr_env). Falls back to "python" or "python3".
func GetPythonPath() (string, error) {
	if runtime.GOOS == "windows" {
		// Try to find "Digital Worker RPA Platform" python from registry
		cmd := exec.Command("reg", "query", `HKLM\SOFTWARE\数字员工平台`, "/v", "PythonPath")
		output, err := cmd.CombinedOutput()
		if err == nil {
			// Output format:
			// HKEY_LOCAL_MACHINE\SOFTWARE\数字员工平台
			//     PythonPath    REG_SZ    C:\Path\To\Python
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "PythonPath") && strings.Contains(line, "REG_SZ") {
					// The path might contain spaces, so we join everything after REG_SZ
					idx := strings.Index(line, "REG_SZ")
					if idx != -1 {
						path := strings.TrimSpace(line[idx+len("REG_SZ"):
						])
						exePath := filepath.Join(path, "python.exe")
						if _, err := os.Stat(exePath); err == nil {
							return exePath, nil
						}
					}
				}
			}
		}
		// Fallback
		return "python", nil
	}

	// Linux / Other
	ocrEnvPython := filepath.Join("/opt", "ocr_env", "bin", "python")
	if _, err := os.Stat(ocrEnvPython); err == nil {
		return ocrEnvPython, nil
	}

	return "python3", nil
}
