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
	"sync"
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

func TestFindResourceFuzzy(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "archive_fuzzy_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_archive_fuzzy.db")
	if err := debug.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	p := &models.SharedProxy{
		OriginalURL: "http://example.com",
		RemotePort:  10082,
		Ctx:         context.Background(),
	}

	sessionID, err := StartSession(p, "Fuzzy Session")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	defer StopSession(p)

	// Record an image with a specific query parameter
	imageURL := "http://example.com/images/navbg.png?version=123"
	req := httptest.NewRequest("GET", imageURL, nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "image/png")
	Record(p, req, resp, []byte(""), []byte("pngdata"))
	time.Sleep(100 * time.Millisecond)

	// Seek using different query parameter (should match fuzzy)
	now := time.Now().UnixNano()
	res, err := FindResource(sessionID, "http://example.com/images/navbg.png?version=456", now)
	if err != nil {
		t.Fatalf("FindResource failed to find fuzzy match: %v", err)
	}
	if res.ResponseBody != "cG5nZGF0YQ==" {
		t.Errorf("expected cG5nZGF0YQ==, got %s", res.ResponseBody)
	}

	// Seek using no query parameter (should also match fuzzy)
	res2, err := FindResource(sessionID, "http://example.com/images/navbg.png", now)
	if err != nil {
		t.Fatalf("FindResource failed to find fuzzy match without query: %v", err)
	}
	if res2.ResponseBody != "cG5nZGF0YQ==" {
		t.Errorf("expected cG5nZGF0YQ==, got %s", res2.ResponseBody)
	}
}

func TestSimulateRender(t *testing.T) {
	// 1. Setup mux
	mux := http.NewServeMux()
	p := &models.SharedProxy{
		OriginalURL: "http://example.com",
		RemotePort:  10082,
		Ctx:         context.Background(),
	}
	RegisterArchiveRoutes(mux, func() []*models.SharedProxy { return []*models.SharedProxy{p} })

	// Request missing CSS asset using collapsed slash URL
	req1 := httptest.NewRequest("GET", "/archive/view/somesession/12345/http:/example.com/missing.css", nil)
	rr1 := httptest.NewRecorder()
	mux.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("expected 200 OK for simulated missing CSS, got %d", rr1.Code)
	}
	if !strings.Contains(rr1.Body.String(), "simulated placeholder") {
		t.Errorf("expected simulated placeholder in body, got: %s", rr1.Body.String())
	}

	// Request missing Image asset using collapsed slash URL
	req2 := httptest.NewRequest("GET", "/archive/view/somesession/12345/http:/example.com/missing.png", nil)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("expected 200 OK for simulated missing image, got %d", rr2.Code)
	}
	if rr2.Header().Get("Content-Type") != "image/png" {
		t.Errorf("expected image/png content type, got: %s", rr2.Header().Get("Content-Type"))
	}
}

func TestPrefetchAssets(t *testing.T) {
	// 1. Setup temporary database
	tmpDir, err := os.MkdirTemp("", "archive_prefetch_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_archive_prefetch.db")
	if err := debug.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	// 2. Start a mock server to serve the assets
	var cssFetched, imgFetched, fontFetched int
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/styles.css" {
			cssFetched++
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte(`body { background: url('/bg.png'); font-family: url("fonts/font.woff2"); }`))
		} else if r.URL.Path == "/bg.png" {
			imgFetched++
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("fake-png-data"))
		} else if r.URL.Path == "/fonts/font.woff2" {
			fontFetched++
			w.Header().Set("Content-Type", "font/woff2")
			w.Write([]byte("fake-woff-data"))
		}
	}))
	defer ts.Close()

	// 3. Define the HTML and SharedProxy
	html := `
<html>
<head>
	<link rel="stylesheet" href="/styles.css">
	<img src="/bg.png">
</head>
<body>
</body>
</html>
`

	p := &models.SharedProxy{
		OriginalURL: ts.URL,
		RemotePort:  10085,
		Ctx:         context.Background(),
	}

	sessionID, err := StartSession(p, "Prefetch Session")
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	defer StopSession(p)

	// 4. Trigger prefetch
	origReq := httptest.NewRequest("GET", ts.URL+"/index.html", nil)
	prefetchAssets(sessionID, origReq, []byte(html))

	// Sleep to allow async fetching and recursive CSS prefetching to complete
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	cCSS := cssFetched
	cImg := imgFetched
	cFont := fontFetched
	mu.Unlock()

	if cCSS != 1 {
		t.Errorf("expected CSS to be fetched once, got %d", cCSS)
	}
	if cImg != 1 {
		t.Errorf("expected Image to be fetched once (via HTML or CSS deduplicated), got %d", cImg)
	}
	if cFont != 1 {
		t.Errorf("expected Font to be fetched once (via CSS), got %d", cFont)
	}
}

