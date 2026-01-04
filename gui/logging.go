package gui

import (
	"fmt"
	"io"
	"log"
	"os"
)

// safeMultiWriter writes to multiple writers, ignoring errors from individual writers
type safeMultiWriter struct {
	writers []io.Writer
}

func (t *safeMultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		_, _ = w.Write(p) // Ignore errors (e.g. from closed stdout)
	}
	return len(p), nil
}

func setupLogging() {
	logFile, err := os.OpenFile("vpn-share-tool.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		// Just print to stdout if file fails
		fmt.Printf("Failed to open log file: %v\n", err)
	} else {
		// defer logFile.Close() // This logic in Run() was deferring inside Run, which is fine.
		// But here we are returning. We can let the OS close it on exit, or return the closer.
		// For simplicity, we assume app runs until exit.
		log.SetOutput(&safeMultiWriter{writers: []io.Writer{os.Stdout, logFile}})
	}
}
