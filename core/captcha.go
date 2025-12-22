package core

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

//go:embed ocr_solver.py
var ocrSolverScript []byte

var (
	captchaSolutions = make(map[string]string) // Map[ClientIP] -> Solution
	captchaLock      sync.RWMutex
)

// StoreCaptchaSolution saves the solution for a client
func StoreCaptchaSolution(clientIP, solution string) {
	captchaLock.Lock()
	defer captchaLock.Unlock()
	captchaSolutions[clientIP] = solution
	log.Printf("Stored captcha solution for %s: %s", clientIP, solution)
}

// ClearCaptchaSolution removes the stored solution for a client
func ClearCaptchaSolution(clientIP string) {
	captchaLock.Lock()
	defer captchaLock.Unlock()
	delete(captchaSolutions, clientIP)
}

// GetCaptchaSolution retrieves and deletes the solution (one-time use)
func GetCaptchaSolution(clientIP string) string {
	captchaLock.Lock()
	defer captchaLock.Unlock()
	if sol, ok := captchaSolutions[clientIP]; ok {
		// will be deleted in ClearCaptchaSolution
		return sol
	}
	return ""
}

func getPythonPath() (string, error) {
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
					parts := strings.Fields(line)
					if len(parts) >= 3 {
						// The path might contain spaces, so we join everything after REG_SZ
						// Fields: [PythonPath, REG_SZ, C:\Path\To\Python]
						// Actually Fields splits by space.
						// We need to find "REG_SZ" and take the rest.
						idx := strings.Index(line, "REG_SZ")
						if idx != -1 {
							path := strings.TrimSpace(line[idx+len("REG_SZ"):])
							exePath := filepath.Join(path, "python.exe")
							if _, err := os.Stat(exePath); err == nil {
								return exePath, nil
							}
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

func SolveCaptcha(imgData []byte) string {
	if DiscoveryServerURL != "" {
		log.Printf("trying to use server to solve...")

		// 1. Format the endpoint URL
		url := fmt.Sprintf("%s/solve-captcha", DiscoveryServerURL)
		client := GetHTTPClient()
		// 2. Post the raw bytes
		resp, err := client.Post(url, "application/octet-stream", bytes.NewBuffer(imgData))
		if err != nil {
			log.Printf("server request failed: %v", err)
			return SolveCaptchaLocal(imgData)
		}
		defer resp.Body.Close()

		// 3. Read the response from the server
		solution, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("failed to read server response: %v", err)
			return SolveCaptchaLocal(imgData)
		}

		return string(solution)
	}

	log.Printf("no discovery server. trying locally...")
	return SolveCaptchaLocal(imgData)
}

// SolveCaptchaLocal attempts to solve the image locally using Python.
func SolveCaptchaLocal(imgData []byte) string {
	log.Printf("Solving captcha locally... (%d bytes)", len(imgData))

	// 1. Create temp script file
	tmpFile, err := os.CreateTemp("", "ocr_solver_*.py")
	if err != nil {
		log.Printf("Failed to create temp solver script: %v", err)
		return ""
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(ocrSolverScript); err != nil {
		log.Printf("Failed to write temp solver script: %v", err)
		return ""
	}
	tmpFile.Close()

	// 2. Get Python Path
	pythonPath, _ := getPythonPath()

	// 3. Run Script
	cmd := exec.Command(pythonPath, tmpFile.Name())
	cmd.SysProcAttr = getSysProcAttr()
	cmd.Stdin = bytes.NewReader(imgData)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Captcha solver failed: %v. Stderr: %s", err, stderr.String())
		return ""
	}

	result := strings.TrimSpace(out.String())
	log.Printf("Captcha solved: '%s'", result)
	return result
}
