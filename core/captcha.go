package core

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/soda92/vpn-share-tool/common"
)

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

// GetCaptchaSolution retrieves the solution for a client
func GetCaptchaSolution(clientIP string) string {
	captchaLock.Lock()
	defer captchaLock.Unlock()
	if sol, ok := captchaSolutions[clientIP]; ok {
		// will be deleted in ClearCaptchaSolution
		return sol
	}
	return ""
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
			return common.SolveCaptchaLocal(imgData)
		}
		defer resp.Body.Close()

		// 3. Read the response from the server
		solution, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("failed to read server response: %v", err)
			return common.SolveCaptchaLocal(imgData)
		}

		return string(solution)
	}

	log.Printf("no discovery server. trying locally...")
	return common.SolveCaptchaLocal(imgData)
}
