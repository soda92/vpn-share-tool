package archive

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.etcd.io/bbolt"
	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/models"
)

const (
	metadataBucketName = "archive_sessions_metadata"
)

type SessionMetadata struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	ProxyURL  string    `json:"proxy_url"`
}

type ArchivedResponse struct {
	Timestamp       time.Time           `json:"timestamp"`
	Method          string              `json:"method"`
	URL             string              `json:"url"`
	RequestHeaders  http.Header         `json:"request_headers"`
	RequestBody     string              `json:"request_body"`
	ResponseStatus  int                 `json:"response_status"`
	ResponseHeaders http.Header         `json:"response_headers"`
	ResponseBody    string              `json:"response_body"`
	IsBase64        bool                `json:"is_base64"`
}

var (
	// Map to track active recording proxies and synchronize accesses
	recordingProxies     = make(map[int]string)
	recordingProxiesLock sync.Mutex
)

// StartSession initializes a recording session for a proxy port
func StartSession(p *models.SharedProxy, name string) (string, error) {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	if p.IsRecording {
		return p.ActiveSessionID, fmt.Errorf("proxy is already recording session %s", p.ActiveSessionID)
	}

	db := debug.GetDB()
	if db == nil {
		return "", fmt.Errorf("debug database not initialized")
	}

	sessionID := uuid.New().String()

	err := db.Update(func(tx *bbolt.Tx) error {
		// Create the metadata bucket if it doesn't exist
		metaBucket, err := tx.CreateBucketIfNotExists([]byte(metadataBucketName))
		if err != nil {
			return err
		}

		// Create a bucket for the session resources
		_, err = tx.CreateBucketIfNotExists([]byte("archive_session_" + sessionID))
		if err != nil {
			return err
		}

		// Save metadata
		meta := SessionMetadata{
			ID:        sessionID,
			Name:      name,
			CreatedAt: time.Now(),
			ProxyURL:  p.OriginalURL,
		}
		data, err := json.Marshal(meta)
		if err != nil {
			return err
		}

		return metaBucket.Put([]byte(sessionID), data)
	})

	if err != nil {
		return "", err
	}

	p.IsRecording = true
	p.ActiveSessionID = sessionID

	recordingProxiesLock.Lock()
	recordingProxies[p.RemotePort] = sessionID
	recordingProxiesLock.Unlock()

	log.Printf("Started archiving session '%s' (ID: %s) on port %d", name, sessionID, p.RemotePort)
	return sessionID, nil
}

// StopSession stops recording for a proxy port
func StopSession(p *models.SharedProxy) error {
	p.Mu.Lock()
	defer p.Mu.Unlock()

	if !p.IsRecording {
		return fmt.Errorf("proxy is not currently recording")
	}

	log.Printf("Stopped archiving session %s on port %d", p.ActiveSessionID, p.RemotePort)

	p.IsRecording = false
	p.ActiveSessionID = ""

	recordingProxiesLock.Lock()
	delete(recordingProxies, p.RemotePort)
	recordingProxiesLock.Unlock()

	return nil
}

// Record captures the HTTP request and response into the active session bucket
func Record(p *models.SharedProxy, req *http.Request, resp *http.Response, reqBody, respBody []byte) {
	p.Mu.RLock()
	isRecording := p.IsRecording
	sessionID := p.ActiveSessionID
	p.Mu.RUnlock()

	if !isRecording || sessionID == "" {
		return
	}

	// 1. Filter out videos based on Content-Type or Extension
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "video/") {
		log.Printf("[Archive] Skipping video content-type: %s", contentType)
		return
	}

	urlPath := strings.ToLower(req.URL.Path)
	ext := filepath.Ext(urlPath)
	videoExts := map[string]bool{
		".mp4":  true,
		".webm": true,
		".mov":  true,
		".mkv":  true,
		".avi":  true,
		".flv":  true,
		".wmv":  true,
		".m4v":  true,
		".3gp":  true,
	}
	if videoExts[ext] {
		log.Printf("[Archive] Skipping video file extension: %s", ext)
		return
	}

	// Capture response details
	timestamp := time.Now()
	method := req.Method
	urlStr := req.URL.String()

	reqHeaders := make(http.Header)
	for k, v := range req.Header {
		reqHeaders[k] = v
	}

	respStatus := resp.StatusCode
	respHeaders := make(http.Header)
	for k, v := range resp.Header {
		respHeaders[k] = v
	}

	// Copy body asynchronously to database
	go func() {
		db := debug.GetDB()
		if db == nil {
			return
		}

		isBase64 := false
		var responseBody string

		if strings.HasPrefix(contentType, "image/") || !utf8.Valid(respBody) {
			responseBody = base64.StdEncoding.EncodeToString(respBody)
			isBase64 = true
		} else {
			responseBody = string(respBody)
		}

		entry := ArchivedResponse{
			Timestamp:       timestamp,
			Method:          method,
			URL:             urlStr,
			RequestHeaders:  reqHeaders,
			RequestBody:     string(reqBody),
			ResponseStatus:  respStatus,
			ResponseHeaders: respHeaders,
			ResponseBody:    responseBody,
			IsBase64:        isBase64,
		}

		err := db.Update(func(tx *bbolt.Tx) error {
			bucketName := "archive_session_" + sessionID
			b := tx.Bucket([]byte(bucketName))
			if b == nil {
				return fmt.Errorf("archive session bucket %s not found", bucketName)
			}

			// Key format: url + "#" + timestamp_nanoseconds
			key := urlStr + "#" + strconv.FormatInt(timestamp.UnixNano(), 10)
			value, err := json.Marshal(entry)
			if err != nil {
				return err
			}

			return b.Put([]byte(key), value)
		})

		if err != nil {
			log.Printf("[Archive] Error writing resource to DB: %v", err)
		} else {
			log.Printf("[Archive] Recorded resource: %s %s (%d bytes)", method, urlStr, len(respBody))
		}
	}()
}

// ListSessions returns a list of all saved archive sessions
func ListSessions() ([]SessionMetadata, error) {
	db := debug.GetDB()
	if db == nil {
		return nil, fmt.Errorf("debug database not initialized")
	}

	var list []SessionMetadata
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(metadataBucketName))
		if b == nil {
			return nil // No sessions created yet
		}

		return b.ForEach(func(k, v []byte) error {
			var meta SessionMetadata
			if err := json.Unmarshal(v, &meta); err == nil {
				list = append(list, meta)
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return list, nil
}

// DeleteSession deletes all data associated with a session
func DeleteSession(sessionID string) error {
	db := debug.GetDB()
	if db == nil {
		return fmt.Errorf("debug database not initialized")
	}

	return db.Update(func(tx *bbolt.Tx) error {
		// 1. Remove from metadata bucket
		metaBucket := tx.Bucket([]byte(metadataBucketName))
		if metaBucket != nil {
			if err := metaBucket.Delete([]byte(sessionID)); err != nil {
				return err
			}
		}

		// 2. Delete the session bucket itself
		bucketName := "archive_session_" + sessionID
		if tx.Bucket([]byte(bucketName)) != nil {
			return tx.DeleteBucket([]byte(bucketName))
		}
		return nil
	})
}
