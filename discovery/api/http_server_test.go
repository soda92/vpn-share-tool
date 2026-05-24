package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyLocalFileHash(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_hash_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	content := []byte("hello world")
	h := sha256.New()
	h.Write(content)
	expectedHash := hex.EncodeToString(h.Sum(nil))

	filePath := filepath.Join(tempDir, "test.exe")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. Correct hash
	if err := verifyLocalFileHash(filePath, expectedHash); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	// 2. Incorrect hash
	if err := verifyLocalFileHash(filePath, "wronghash"); err == nil {
		t.Error("expected error for wrong hash, got nil")
	}

	// 3. Test caching (change file content but keep path, size, and mod time; cached hash should hit and succeed)
	fi, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	originalModTime := fi.ModTime()

	// Modify the file on disk to have different content but identical size (11 bytes)
	newContent := []byte("world hello")
	if err := os.WriteFile(filePath, newContent, 0644); err != nil {
		t.Fatalf("failed to rewrite file: %v", err)
	}

	// Restore original modification time
	if err := os.Chtimes(filePath, originalModTime, originalModTime); err != nil {
		t.Fatalf("failed to restore mod time: %v", err)
	}

	// Since it's cached and size/modtime match, it should still succeed for the original expectedHash
	if err := verifyLocalFileHash(filePath, expectedHash); err != nil {
		t.Errorf("expected cached success, got error: %v", err)
	}

	// 4. Test cache invalidation (change size; cached hash should NOT hit and should fail)
	if err := os.WriteFile(filePath, []byte("changed content size"), 0644); err != nil {
		t.Fatalf("failed to rewrite file with different size: %v", err)
	}

	if err := verifyLocalFileHash(filePath, expectedHash); err == nil {
		t.Error("expected error after size change (cache invalidation), got nil")
	}

	// 5. Test cache invalidation after clearing cache entry
	verifiedHashesCacheLock.Lock()
	delete(verifiedHashesCache, filePath)
	verifiedHashesCacheLock.Unlock()

	if err := verifyLocalFileHash(filePath, expectedHash); err == nil {
		t.Error("expected error after clearing cache, got nil")
	}
}

func TestHandleVersionRequest(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test_share_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldSharePath := SharePath
	SharePath = tempDir
	defer func() { SharePath = oldSharePath }()

	// Create a dummy app file with sha256
	appFile := "vpn-share-tool-app_v40b.zip"
	appData := []byte("app binary data zip")
	h := sha256.New()
	h.Write(appData)
	appHash := hex.EncodeToString(h.Sum(nil))

	if err := os.WriteFile(filepath.Join(tempDir, appFile), appData, 0644); err != nil {
		t.Fatalf("failed to write app file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, appFile+".sha256"), []byte(appHash), 0644); err != nil {
		t.Fatalf("failed to write app hash file: %v", err)
	}

	// Create a dummy updater file (WITHOUT sha256)
	updaterFile := "vpn-share-tool_v40c.exe"
	updaterData := []byte("updater binary data exe")
	if err := os.WriteFile(filepath.Join(tempDir, updaterFile), updaterData, 0644); err != nil {
		t.Fatalf("failed to write updater file: %v", err)
	}

	// Now query /latest-app-version
	reqApp, err := http.NewRequest("GET", "/latest-app-version?format=zip", nil)
	if err != nil {
		t.Fatal(err)
	}
	rrApp := httptest.NewRecorder()
	handlerApp := http.HandlerFunc(handleLatestAppVersion)
	handlerApp.ServeHTTP(rrApp, reqApp)

	if rrApp.Code != http.StatusOK {
		t.Errorf("expected 200 OK for latest-app-version, got %v: %s", rrApp.Code, rrApp.Body.String())
	}

	var appResp updateInfo
	if err := json.Unmarshal(rrApp.Body.Bytes(), &appResp); err != nil {
		t.Fatalf("failed to unmarshal app response: %v", err)
	}

	if appResp.Version != "v40b" || appResp.Sha256 != appHash || appResp.URL != "/download/vpn-share-tool-app_v40b.zip" {
		t.Errorf("unexpected latest-app-version response: %+v", appResp)
	}

	// Now query /latest-version (updater)
	reqUpdater, err := http.NewRequest("GET", "/latest-version", nil)
	if err != nil {
		t.Fatal(err)
	}
	rrUpdater := httptest.NewRecorder()
	handlerUpdater := http.HandlerFunc(handleLatestVersion)
	handlerUpdater.ServeHTTP(rrUpdater, reqUpdater)

	if rrUpdater.Code != http.StatusOK {
		t.Errorf("expected 200 OK for latest-version, got %v: %s", rrUpdater.Code, rrUpdater.Body.String())
	}

	var updaterResp updateInfo
	if err := json.Unmarshal(rrUpdater.Body.Bytes(), &updaterResp); err != nil {
		t.Fatalf("failed to unmarshal updater response: %v", err)
	}

	// For the updater, it has no sha256 file, so it should be returned successfully with empty Sha256!
	if updaterResp.Version != "v40c" || updaterResp.Sha256 != "" || updaterResp.URL != "/download/vpn-share-tool_v40c.exe" {
		t.Errorf("unexpected latest-version response: %+v", updaterResp)
	}
}
