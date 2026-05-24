package api

import (
	"crypto/sha256"
	"encoding/hex"
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

	// 3. Test caching (change file content but keep path; cached hash should hit and succeed)
	// Modify the file on disk to have different content
	if err := os.WriteFile(filePath, []byte("changed content"), 0644); err != nil {
		t.Fatalf("failed to rewrite file: %v", err)
	}

	// Since it's cached, it should still succeed for the original expectedHash
	if err := verifyLocalFileHash(filePath, expectedHash); err != nil {
		t.Errorf("expected cached success, got error: %v", err)
	}

	// But if we clear the cache entry, it should re-evaluate and fail for the original expectedHash
	verifiedHashesCacheLock.Lock()
	delete(verifiedHashesCache, filePath)
	verifiedHashesCacheLock.Unlock()

	if err := verifyLocalFileHash(filePath, expectedHash); err == nil {
		t.Error("expected error after clearing cache, got nil")
	}
}
