package gui

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/soda92/vpn-share-tool/core"
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

type remoteWriter struct {
	buffer chan string
	ws     *websocket.Conn
}

func newRemoteWriter() *remoteWriter {
	rw := &remoteWriter{
		buffer: make(chan string, 100),
	}
	go rw.flushLoop()
	return rw
}

func (r *remoteWriter) Write(p []byte) (n int, err error) {
	select {
	case r.buffer <- string(p):
	default:
		// Drop log if buffer is full
	}
	return len(p), nil
}

func (r *remoteWriter) flushLoop() {
	ticker := time.NewTicker(2 * time.Second) // Flush every 2 seconds or immediately on buffer full?
	// Actually we can stream faster with WS.
	
	for {
		select {
		case line := <-r.buffer:
			r.send(line)
		case <-ticker.C:
			// Ping / keepalive if needed, or just let TCP handle it.
		}
	}
}

func (r *remoteWriter) send(logLine string) {
	if core.DiscoveryServerURL == "" || core.MyIP == "" {
		return // Not ready
	}

	if r.ws == nil {
		r.connect()
	}

	if r.ws == nil {
		return // Connection failed
	}

	// Prepare payload
	address := core.MyIP
	if core.APIPort != 0 {
		address = fmt.Sprintf("%s:%d", core.MyIP, core.APIPort)
	}
	
	data := map[string]string{
		"address": address,
		"logs":    logLine,
	}

	if err := r.ws.WriteJSON(data); err != nil {
		// fmt.Printf("WS Write error: %v\n", err)
		r.ws.Close()
		r.ws = nil
		// Drop this log line or retry? Drop for simplicity to avoid blocking.
	}
}

func (r *remoteWriter) connect() {
	// Construct WS URL
	target := core.DiscoveryServerURL
	if strings.HasPrefix(target, "http://") {
		target = "ws://" + strings.TrimPrefix(target, "http://")
	} else if strings.HasPrefix(target, "https://") {
		target = "wss://" + strings.TrimPrefix(target, "https://")
	} else {
		target = "ws://" + target // Assume insecure if unknown
	}
	target = strings.TrimRight(target, "/") + "/upload-logs"

	// Connect
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	c, _, err := dialer.Dial(target, nil)
	if err != nil {
		// fmt.Printf("WS Dial error: %v\n", err)
		return
	}
	r.ws = c
}

func setupLogging() {
	var logPath string
	if home, err := os.UserHomeDir(); err == nil {
		logPath = filepath.Join(home, ".vpn-share-tool", "vpn-share-tool.log")
		_ = os.MkdirAll(filepath.Dir(logPath), 0755)
	} else {
		logPath = "vpn-share-tool.log"
	}

	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		// Just print to stdout if file fails
		fmt.Printf("Failed to open log file: %v\n", err)
		log.SetOutput(&safeMultiWriter{writers: []io.Writer{os.Stdout, newRemoteWriter()}})
	} else {
		// defer logFile.Close() // This logic in Run() was deferring inside Run, which is fine.
		// But here we are returning. We can let the OS close it on exit, or return the closer.
		// For simplicity, we assume app runs until exit.
		log.SetOutput(&safeMultiWriter{writers: []io.Writer{os.Stdout, logFile, newRemoteWriter()}})
	}
}
