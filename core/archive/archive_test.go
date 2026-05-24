package archive

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/models"
	"go.etcd.io/bbolt"
)

func TestArchiveSessionLifecycle(t *testing.T) {
	// 1. Setup temporary database
	tmpDir, err := os.MkdirTemp("", "archive_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_archive.db")
	if err := debug.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	p := &models.SharedProxy{
		OriginalURL: "http://example.com",
		RemotePort:  10081,
		Ctx:         context.Background(),
	}

	// 2. Test StartSession
	sessionID, err := StartSession(p, "Test Session")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}

	if sessionID == "" {
		t.Errorf("expected non-empty session ID")
	}

	if !p.IsRecording || p.ActiveSessionID != sessionID {
		t.Errorf("proxy recording state not updated correctly")
	}

	// 3. Test duplicate start error
	_, err = StartSession(p, "Another Session")
	if err == nil {
		t.Errorf("expected error when starting an already recording session")
	}

	// 4. Test ListSessions
	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != sessionID || sessions[0].Name != "Test Session" {
		t.Errorf("ListSessions returned unexpected sessions: %+v", sessions)
	}

	// 5. Test StopSession
	err = StopSession(p)
	if err != nil {
		t.Fatalf("StopSession failed: %v", err)
	}

	if p.IsRecording || p.ActiveSessionID != "" {
		t.Errorf("proxy recording state not cleared correctly")
	}

	// 6. Test DeleteSession
	err = DeleteSession(sessionID)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	sessions, err = ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed after delete: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after deletion, got %d", len(sessions))
	}
}

func TestArchiveRecordingAndPlayback(t *testing.T) {
	// 1. Setup temporary database
	tmpDir, err := os.MkdirTemp("", "archive_playback_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_archive_playback.db")
	if err := debug.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	p := &models.SharedProxy{
		OriginalURL: "http://example.com",
		RemotePort:  10082,
		Ctx:         context.Background(),
	}

	// Start recording
	sessionID, err := StartSession(p, "Playback Session")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	defer StopSession(p)

	// Define test resource details
	targetURL := "http://example.com/styles.css"
	reqBody := []byte("")
	respBody := []byte(".body { background: purple; }")

	req := httptest.NewRequest("GET", targetURL, nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "text/css")

	// 2. Perform Record
	Record(p, req, resp, reqBody, respBody)

	// Sleep briefly to let the asynchronous recording goroutine write to bbolt
	time.Sleep(100 * time.Millisecond)

	// 3. Test Playback exact/nearest lookup
	nowNano := time.Now().UnixNano()
	archived, err := FindResource(sessionID, targetURL, nowNano)
	if err != nil {
		t.Fatalf("FindResource failed: %v", err)
	}

	if archived.ResponseStatus != http.StatusOK {
		t.Errorf("expected status 200, got %d", archived.ResponseStatus)
	}

	if archived.ResponseBody != string(respBody) {
		t.Errorf("expected body '%s', got '%s'", string(respBody), archived.ResponseBody)
	}

	// 4. Test video exclusion
	videoURL := "http://example.com/video.mp4"
	videoReq := httptest.NewRequest("GET", videoURL, nil)
	videoResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}
	videoResp.Header.Set("Content-Type", "video/mp4")

	Record(p, videoReq, videoResp, []byte(""), []byte("mp4binarybytes"))
	time.Sleep(100 * time.Millisecond)

	_, err = FindResource(sessionID, videoURL, nowNano)
	if err == nil || !strings.Contains(err.Error(), "no archive matches found") {
		t.Errorf("expected video resource to be ignored/excluded from archive")
	}
}

func TestFindClosestTimestamp(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "archive_closest_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_archive_closest.db")
	if err := debug.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	p := &models.SharedProxy{
		OriginalURL: "http://example.com",
		RemotePort:  10083,
		Ctx:         context.Background(),
	}

	sessionID, err := StartSession(p, "Closest Session")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	defer StopSession(p)

	targetURL := "http://example.com/data.json"
	req := httptest.NewRequest("GET", targetURL, nil)

	// Record snapshot 1
	resp1 := &http.Response{StatusCode: 200, Header: make(http.Header)}
	Record(p, req, resp1, []byte(""), []byte("snapshot_1"))
	time.Sleep(50 * time.Millisecond)

	// Record snapshot 2 after a small delay
	resp2 := &http.Response{StatusCode: 201, Header: make(http.Header)}
	Record(p, req, resp2, []byte(""), []byte("snapshot_2"))
	time.Sleep(100 * time.Millisecond)

	// Get timestamps
	db := debug.GetDB()
	var timestamps []int64
	db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("archive_session_" + sessionID))
		c := b.Cursor()
		prefix := targetURL + "#"
		for k, _ := c.Seek([]byte(prefix)); k != nil && strings.HasPrefix(string(k), prefix); k, _ = c.Next() {
			parts := strings.Split(string(k), "#")
			ts, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
			timestamps = append(timestamps, ts)
		}
		return nil
	})

	if len(timestamps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(timestamps))
	}

	// Query exactly at snapshot 1 timestamp
	archived1, err := FindResource(sessionID, targetURL, timestamps[0])
	if err != nil {
		t.Fatalf("FindResource failed: %v", err)
	}
	if archived1.ResponseBody != "snapshot_1" {
		t.Errorf("expected snapshot_1, got %s", archived1.ResponseBody)
	}

	// Query exactly at snapshot 2 timestamp
	archived2, err := FindResource(sessionID, targetURL, timestamps[1])
	if err != nil {
		t.Fatalf("FindResource failed: %v", err)
	}
	if archived2.ResponseBody != "snapshot_2" {
		t.Errorf("expected snapshot_2, got %s", archived2.ResponseBody)
	}

	// Query intermediate timestamp (closer to snapshot 1)
	queryTime1 := timestamps[0] + 10
	archivedIntermediate1, err := FindResource(sessionID, targetURL, queryTime1)
	if err != nil {
		t.Fatalf("FindResource failed: %v", err)
	}
	if archivedIntermediate1.ResponseBody != "snapshot_1" {
		t.Errorf("expected intermediate closer to 1 to yield snapshot_1, got %s", archivedIntermediate1.ResponseBody)
	}

	// Query intermediate timestamp (closer to snapshot 2)
	queryTime2 := timestamps[1] - 10
	archivedIntermediate2, err := FindResource(sessionID, targetURL, queryTime2)
	if err != nil {
		t.Fatalf("FindResource failed: %v", err)
	}
	if archivedIntermediate2.ResponseBody != "snapshot_2" {
		t.Errorf("expected intermediate closer to 2 to yield snapshot_2, got %s", archivedIntermediate2.ResponseBody)
	}
}

func TestPlaybackFallback(t *testing.T) {
	// 1. Setup temporary database
	tmpDir, err := os.MkdirTemp("", "archive_fallback_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_archive_fallback.db")
	if err := debug.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	p := &models.SharedProxy{
		OriginalURL: "http://192.168.1.230:8080",
		RemotePort:  10082,
		Ctx:         context.Background(),
	}

	sessionID, err := StartSession(p, "Fallback Session")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	defer StopSession(p)

	// 2. Record stylesheet relative to the proxy
	cssURL := "http://192.168.1.230:8080/Content/platform.css"
	reqBody := []byte("")
	respBody := []byte("body { background: red; }")

	cssReq := httptest.NewRequest("GET", cssURL, nil)
	cssResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}
	cssResp.Header.Set("Content-Type", "text/css")

	Record(p, cssReq, cssResp, reqBody, respBody)
	time.Sleep(100 * time.Millisecond)

	// 3. Setup http.ServeMux and register route
	mux := http.NewServeMux()
	RegisterArchiveRoutes(mux, func() []*models.SharedProxy { return []*models.SharedProxy{p} })

	// 4. Perform a request to an absolute-path relative asset WITHOUT /archive/... prefix,
	// but WITH the Referer header pointing to an archived page.
	targetTimestamp := time.Now().UnixNano()
	req := httptest.NewRequest("GET", "/Content/platform.css", nil)
	refererURL := fmt.Sprintf("http://127.0.0.1:10081/archive/view/%s/%d/http://192.168.1.230:8080/index.html", sessionID, targetTimestamp)
	req.Header.Set("Referer", refererURL)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	// Verify the response
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/css") {
		t.Errorf("expected Content-Type text/css, got %s", contentType)
	}

	body := rr.Body.String()
	if body != "body { background: red; }" {
		t.Errorf("expected css body, got %s", body)
	}

	// 5. Perform a request WITHOUT Referer header - should return 404
	req2 := httptest.NewRequest("GET", "/Content/platform.css", nil)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotFound {
		t.Errorf("expected 404 for request without referer, got %d", rr2.Code)
	}
}
