package common

import (
	"bytes"
	_ "embed"
	"log"
	"os"
	"os/exec"
	"strings"
)

//go:embed ocr_solver.py
var ocrSolverScript []byte

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

	pythonPath, err := GetPythonPath()
	if err != nil {
		log.Printf("Failed to get python path: %v", err)
		return ""
	}

	// 3. Run Script
	cmd := exec.Command(pythonPath, tmpFile.Name())
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
