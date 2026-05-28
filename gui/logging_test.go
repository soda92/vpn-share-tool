package gui

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRotateLogIfNeeded(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "log_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	logPath := filepath.Join(tempDir, "vpn-share-tool.log")

	// 1. Write small data (should NOT trigger rotation)
	smallData := []byte("hello")
	if err := os.WriteFile(logPath, smallData, 0666); err != nil {
		t.Fatalf("failed to write log: %v", err)
	}

	rotateLogIfNeeded(logPath)

	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file should exist: %v", err)
	}
	if fi.Size() != int64(len(smallData)) {
		t.Errorf("expected size %d, got %d", len(smallData), fi.Size())
	}

	// 2. Write data exceeding 10MB to trigger rotation
	largeSize := 10*1024*1024 + 1
	largeData := make([]byte, largeSize)
	for i := range largeData {
		largeData[i] = 'A'
	}
	if err := os.WriteFile(logPath, largeData, 0666); err != nil {
		t.Fatalf("failed to write large log: %v", err)
	}

	rotateLogIfNeeded(logPath)

	// Log file should be truncated to 0 size
	fi, err = os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file should exist after rotation: %v", err)
	}
	if fi.Size() != 0 {
		t.Errorf("expected log file size to be 0 after rotation, got %d", fi.Size())
	}

	// Backup file .1.gz should exist
	backupPath := logPath + ".1.gz"
	backupFi, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("backup file %s should exist: %v", backupPath, err)
	}
	if backupFi.Size() == 0 {
		t.Error("backup file is empty")
	}

	// Read backup and verify content size
	f, err := os.Open(backupPath)
	if err != nil {
		t.Fatalf("failed to open backup: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("failed to read decompressed data: %v", err)
	}

	if len(decompressed) != largeSize {
		t.Errorf("expected decompressed size %d, got %d", largeSize, len(decompressed))
	}
}
