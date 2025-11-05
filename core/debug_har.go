package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

func handleHar(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		importHar(w, r)
	case http.MethodGet:
		exportHar(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func importHar(w http.ResponseWriter, r *http.Request) {
	var har HAR
	if err := json.NewDecoder(r.Body).Decode(&har); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse HAR file: %v", err), http.StatusBadRequest)
		return
	}

	sessionName := r.URL.Query().Get("name")
	if sessionName == "" {
		sessionName = fmt.Sprintf("Imported Session @ %s", time.Now().Format("2006-01-02 15:04:05"))
	}

	sessionID := uuid.New().String()

	err := db.Update(func(tx *bbolt.Tx) error {
		// Create new bucket for the session
		destBucket, err := tx.CreateBucket([]byte(sessionID))
		if err != nil {
			return err
		}

		for _, entry := range har.Log.Entries {
			requestIDLock.Lock()
			nextRequestID++
			cr := &CapturedRequest{
				ID:             nextRequestID,
				Timestamp:      entry.StartedDateTime,
				Method:         entry.Request.Method,
				URL:            entry.Request.URL,
				RequestHeaders: make(http.Header),
				ResponseStatus: entry.Response.Status,
				ResponseHeaders: make(http.Header),
			}
			requestIDLock.Unlock()

			if entry.Request.PostData != nil {
				cr.RequestBody = entry.Request.PostData.Text
			}
			if entry.Response.Content.Text != "" {
				cr.ResponseBody = entry.Response.Content.Text
			}

			for _, h := range entry.Request.Headers {
				cr.RequestHeaders.Add(h.Name, h.Value)
			}
			for _, h := range entry.Response.Headers {
				cr.ResponseHeaders.Add(h.Name, h.Value)
			}

			if entry.VpnShareToolMetadata != nil {
				cr.Bookmarked = entry.VpnShareToolMetadata.Bookmarked
				cr.Note = entry.VpnShareToolMetadata.Note
			}

			jsonReq, _ := json.Marshal(cr)
			if err := destBucket.Put([]byte(strconv.FormatInt(cr.ID, 10)), jsonReq); err != nil {
				return err
			}
		}

		// Add to metadata
		metaBucket := tx.Bucket([]byte(sessionsMetadataBucket))
		return metaBucket.Put([]byte(sessionID), []byte(sessionName))
	})

	if err != nil {
		log.Printf("Error importing HAR session: %v", err)
		http.Error(w, "Failed to import HAR session", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": sessionID, "name": sessionName})
}

func exportHar(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/debug/sessions/")
	sessionID := strings.TrimSuffix(path, "/har")

	var requests []*CapturedRequest
	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(sessionID))
		if b == nil {
			return fmt.Errorf("session not found")
		}
		return b.ForEach(func(k, v []byte) error {
			var req CapturedRequest
			if err := json.Unmarshal(v, &req); err == nil {
				requests = append(requests, &req)
			}
			return nil
		})
	})

	if err != nil {
		http.NotFound(w, r)
		return
	}

	har := toHAR(requests)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.har\"", sessionID))
	json.NewEncoder(w).Encode(har)
}

func toHAR(requests []*CapturedRequest) HAR {
	entries := make([]Entry, len(requests))
	for i, req := range requests {
		u, _ := url.Parse(req.URL)

		reqHeaders := make([]NameValuePair, 0, len(req.RequestHeaders))
		for name, values := range req.RequestHeaders {
			for _, value := range values {
				reqHeaders = append(reqHeaders, NameValuePair{Name: name, Value: value})
			}
		}

		respHeaders := make([]NameValuePair, 0, len(req.ResponseHeaders))
		for name, values := range req.ResponseHeaders {
			for _, value := range values {
				respHeaders = append(respHeaders, NameValuePair{Name: name, Value: value})
			}
		}

		queryString := make([]NameValuePair, 0)
		for name, values := range u.Query() {
			for _, value := range values {
				queryString = append(queryString, NameValuePair{Name: name, Value: value})
			}
		}

		var postData *PostData
		if req.RequestBody != "" {
			postData = &PostData{
				MimeType: req.RequestHeaders.Get("Content-Type"),
				Text:     req.RequestBody,
			}
		}

		entries[i] = Entry{
			StartedDateTime: req.Timestamp,
			Time:            -1, // Not easily available, set to -1
			Request: Request{
				Method:      req.Method,
				URL:         req.URL,
				HTTPVersion: "HTTP/1.1", // Assumption
				Headers:     reqHeaders,
				QueryString: queryString,
				PostData:    postData,
				BodySize:    int64(len(req.RequestBody)),
			},
			Response: Response{
				Status:      req.ResponseStatus,
				StatusText:  http.StatusText(req.ResponseStatus),
				HTTPVersion: "HTTP/1.1", // Assumption
				Headers:     respHeaders,
				Content: Content{
					Size:     int64(len(req.ResponseBody)),
					MimeType: req.ResponseHeaders.Get("Content-Type"),
					Text:     req.ResponseBody,
				},
				BodySize: int64(len(req.ResponseBody)),
			},
			Cache: make(map[string]interface{}),
			Timings: Timings{Send: -1, Wait: -1, Receive: -1}, // Not available
		}

		if req.Bookmarked || req.Note != "" {
			entries[i].VpnShareToolMetadata = &VpnShareToolMetadata{
				Bookmarked: req.Bookmarked,
				Note:       req.Note,
			}
		}
	}

	return HAR{
		Log: Log{
			Version: "1.2",
			Creator: Creator{Name: "VPN Share Tool", Version: "1.0"},
			Entries: entries,
		},
	}
}
