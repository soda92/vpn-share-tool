package core

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestUnzipAndSHA256(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "updater_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	exeContent := []byte("fake-exe-binary-payload-data-here-12345")
	zipBuf := new(bytes.Buffer)
	zw := zip.NewWriter(zipBuf)

	// Create test file in zip
	f, err := zw.Create("vpn-share-tool.exe")
	if err != nil {
		t.Fatalf("failed to create file in zip: %v", err)
	}
	if _, err := f.Write(exeContent); err != nil {
		t.Fatalf("failed to write content to zip: %v", err)
	}
	zw.Close()

	// Write zip to temp file
	zipPath := filepath.Join(tmpDir, "update.zip")
	if err := os.WriteFile(zipPath, zipBuf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write zip file: %v", err)
	}

	// 1. Verify getFileSHA256
	expectedHash := sha256.Sum256(zipBuf.Bytes())
	expectedHashStr := hex.EncodeToString(expectedHash[:])
	actualHashStr, err := getFileSHA256(zipPath)
	if err != nil {
		t.Fatalf("getFileSHA256 failed: %v", err)
	}
	if actualHashStr != expectedHashStr {
		t.Errorf("getFileSHA256 mismatch: expected %s, got %s", expectedHashStr, actualHashStr)
	}

	// 2. Verify verifySHA256
	if err := verifySHA256(zipPath, expectedHashStr); err != nil {
		t.Errorf("verifySHA256 failed on correct hash: %v", err)
	}
	if err := verifySHA256(zipPath, "wrong-hash"); err == nil {
		t.Error("verifySHA256 succeeded on incorrect hash")
	}

	// 3. Verify unzipFile
	destPath := filepath.Join(tmpDir, "extracted.exe")
	if err := unzipFile(zipPath, destPath); err != nil {
		t.Fatalf("unzipFile failed: %v", err)
	}

	extractedContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read extracted file: %v", err)
	}
	if !bytes.Equal(extractedContent, exeContent) {
		t.Errorf("extracted content mismatch: expected %q, got %q", string(exeContent), string(extractedContent))
	}
}
