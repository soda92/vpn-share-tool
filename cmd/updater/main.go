package main

import (
	"archive/zip"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/soda92/vpn-share-tool/core/register"
	"github.com/soda92/vpn-share-tool/core/resources"
)

const UpdaterVersion = "v40a"

var ServerIPs = []string{"127.0.0.1", "192.168.0.81", "192.168.1.81"}

type updateInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	Sha256  string `json:"sha256"`
}

func main() {
	sourceExe := flag.String("source", "vpn-share-tool.exe", "Path to the main executable to update")
	flag.Parse()

	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get own executable path: %v", err)
	}

	filename := filepath.Base(exePath)

	// If named vpn-share-tool.exe, we are in Bootstrap Mode (started by old client upgrade)
	if strings.EqualFold(filename, "vpn-share-tool.exe") {
		runBootstrapMode(exePath)
		return
	}

	// Otherwise, we are in Upgrade Mode (running as updater.exe)
	runUpgradeMode(*sourceExe)
}

func runBootstrapMode(exePath string) {
	exeDir := filepath.Dir(exePath)
	updaterExe := filepath.Join(exeDir, "updater.exe")

	fmt.Printf("[Bootstrap] Copying self to %s...\n", updaterExe)
	if err := copySelfTo(exePath, updaterExe); err != nil {
		log.Fatalf("[Bootstrap] Failed to copy self: %v", err)
	}

	fmt.Println("[Bootstrap] Launching updater.exe...")
	cmd := exec.Command(updaterExe, "--source", exePath)
	if err := cmd.Start(); err != nil {
		log.Fatalf("[Bootstrap] Failed to start updater.exe: %v", err)
	}

	fmt.Println("[Bootstrap] Exiting bootstrap.")
	os.Exit(0)
}

func copySelfTo(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

func runUpgradeMode(sourceExe string) {
	fmt.Println("========================================")
	fmt.Println("        VPN Share Tool Updater")
	fmt.Println("========================================")

	err := performUpgrade(sourceExe)
	if err != nil {
		fmt.Printf("\n❌ Update Failed: %v\n", err)
		fmt.Println("Press Enter to close...")
		var tmp string
		fmt.Scanln(&tmp)
		os.Exit(1)
	}

	fmt.Println("\n✅ Update completed successfully! Restarting...")
	time.Sleep(1 * time.Second)
}

func performUpgrade(sourceExe string) error {
	// 1. Wait for sourceExe process to release the file lock
	fmt.Println("[1/5] Waiting for main application to exit...")
	err := waitForFileRelease(sourceExe, 15*time.Second)
	if err != nil {
		return fmt.Errorf("main app did not exit in time: %w", err)
	}

	// 2. Discover Discovery Server URL
	fmt.Println("[2/5] Discovering update server...")
	discoveryURL, err := discoverServer(10 * time.Second)
	if err != nil {
		return fmt.Errorf("failed to discover update server: %w", err)
	}
	fmt.Printf("      Server found: %s\n", discoveryURL)

	// 3. Check for Updater self-update
	fmt.Println("[3/5] Checking for updater updates...")
	client := getTLSClient()
	updaterInfo, err := queryLatestVersion(client, discoveryURL, "/latest-version?format=zip")
	if err == nil && updaterInfo != nil && isNewerVersion(updaterInfo.Version, UpdaterVersion) {
		fmt.Printf("      New updater version %s available. Updating updater first...\n", updaterInfo.Version)
		downloadPath := sourceExe
		isZip := strings.HasSuffix(updaterInfo.URL, ".zip")
		if isZip {
			downloadPath = sourceExe + ".zip"
		}

		err = downloadFileWithProgress(client, discoveryURL+updaterInfo.URL, downloadPath)
		if err != nil {
			return fmt.Errorf("failed to download updater update: %w", err)
		}

		err = verifyFileHash(downloadPath, updaterInfo.Sha256)
		if err != nil {
			os.Remove(downloadPath)
			return fmt.Errorf("updater update verification failed: %w", err)
		}

		if isZip {
			fmt.Println("      Extracting updater update...")
			err = extractZip(downloadPath, sourceExe)
			os.Remove(downloadPath)
			if err != nil {
				return fmt.Errorf("failed to extract updater update: %w", err)
			}
		}

		fmt.Println("      Applying updater update...")
		os.Chmod(sourceExe, 0755)

		cmd := exec.Command(sourceExe)
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to restart updater bootstrap: %w", err)
		}
		os.Exit(0)
	}

	// 4. Check for Actual App Update
	fmt.Println("[4/5] Checking for application updates...")
	appInfo, err := queryLatestVersion(client, discoveryURL, "/latest-app-version?format=zip")
	if err != nil {
		return fmt.Errorf("failed to check app update: %w", err)
	}

	if appInfo == nil {
		return fmt.Errorf("no application update found on server")
	}

	// 5. Download App update
	fmt.Printf("[5/5] Downloading version %s...\n", appInfo.Version)
	downloadPath := sourceExe
	isZip := strings.HasSuffix(appInfo.URL, ".zip")
	if isZip {
		downloadPath = sourceExe + ".zip"
	}

	err = downloadFileWithProgress(client, discoveryURL+appInfo.URL, downloadPath)
	if err != nil {
		return fmt.Errorf("failed to download application update: %w", err)
	}

	err = verifyFileHash(downloadPath, appInfo.Sha256)
	if err != nil {
		os.Remove(downloadPath)
		return fmt.Errorf("application verification failed: %w", err)
	}

	if isZip {
		fmt.Println("      Extracting application update...")
		err = extractZip(downloadPath, sourceExe)
		os.Remove(downloadPath)
		if err != nil {
			return fmt.Errorf("failed to extract application update: %w", err)
		}
	}

	fmt.Println("      Starting application...")
	os.Chmod(sourceExe, 0755)

	cmd := exec.Command(sourceExe)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	return nil
}

func waitForFileRelease(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		f, err := os.OpenFile(path, os.O_WRONLY, 0666)
		if err == nil {
			f.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("file lock timeout")
}

func discoverServer(timeout time.Duration) (string, error) {
	urlChan := make(chan string, 1)
	regCfg := register.Config{
		DiscoverySrvPort:  "45679",
		FallbackServerIPs: ServerIPs,
		RootCACert:        resources.RootCACert,
		UpdateDiscoveryURL: func(url string) {
			select {
			case urlChan <- url:
			default:
			}
		},
	}
	go register.Start(regCfg)

	select {
	case url := <-urlChan:
		return url, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("discovery timeout")
	}
}

func getTLSClient() *http.Client {
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(resources.RootCACert)

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: caPool,
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

func queryLatestVersion(client *http.Client, serverURL string, path string) (*updateInfo, error) {
	resp, err := client.Get(serverURL + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	var info updateInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}

func isNewerVersion(v1, v2 string) bool {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	var num1, num2 int
	var suf1, suf2 string

	fmt.Sscanf(v1, "%d%s", &num1, &suf1)
	fmt.Sscanf(v2, "%d%s", &num2, &suf2)

	if num1 != num2 {
		return num1 > num2
	}

	return suf1 > suf2
}

type progressWriter struct {
	total      int64
	downloaded int64
	lastUpdate time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.downloaded += int64(n)

	if time.Since(pw.lastUpdate) > 100*time.Millisecond || pw.downloaded == pw.total {
		pw.lastUpdate = time.Now()
		percentage := float64(pw.downloaded) / float64(pw.total) * 100

		bars := int(percentage / 5)
		barStr := strings.Repeat("=", bars)
		if bars < 20 {
			barStr += ">" + strings.Repeat(" ", 19-bars)
		} else {
			barStr = strings.Repeat("=", 20)
		}

		fmt.Printf("\r      Progress: [%s] %.1f%% (%d/%d MB)",
			barStr, percentage, pw.downloaded/(1024*1024), pw.total/(1024*1024))
	}
	return n, nil
}

func downloadFileWithProgress(client *http.Client, url string, destPath string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: status %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	pw := &progressWriter{
		total:      resp.ContentLength,
		lastUpdate: time.Now(),
	}

	_, err = io.Copy(io.MultiWriter(out, pw), resp.Body)
	fmt.Println()
	if err != nil {
		return err
	}

	return out.Sync()
}

func getFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func verifyFileHash(path string, expectedHash string) error {
	if expectedHash == "" {
		return nil
	}

	actualHash, err := getFileSHA256(path)
	if err != nil {
		return err
	}

	if !strings.EqualFold(actualHash, expectedHash) {
		return fmt.Errorf("hash mismatch! expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

func extractZip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	var targetFile *zip.File
	for _, f := range r.File {
		if filepath.Base(f.Name) == "vpn-share-tool.exe" {
			targetFile = f
			break
		}
	}

	if targetFile == nil && len(r.File) > 0 {
		targetFile = r.File[0]
	}

	if targetFile == nil {
		return fmt.Errorf("no files found in zip")
	}

	rc, err := targetFile.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}
