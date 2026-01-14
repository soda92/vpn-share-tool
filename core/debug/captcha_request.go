package debug

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.etcd.io/bbolt"
)

var (
	nextRequestID int64
	requestIDLock sync.Mutex
)

// CaptureRequest captures the request and response for debugging.
func CaptureRequest(req *http.Request, resp *http.Response, reqBody, respBody []byte) {
	// Extract data synchronously to avoid race conditions as the request object might be reused/invalidated
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

	// Create copies of bodies if they are not already (they are slices, but underlying arrays might be large or shared?)
	// In the calling code (assets.go), they are results of io.ReadAll, so they are distinct allocations.
	// But passing them to goroutine is safe.

	go func() {
		requestIDLock.Lock()
		nextRequestID++
		id := nextRequestID
		requestIDLock.Unlock()

		isBase64 := false
		var responseBody string

		contentType := respHeaders.Get("Content-Type")
		if strings.HasPrefix(contentType, "image/") || !utf8.Valid(respBody) {
			responseBody = base64.StdEncoding.EncodeToString(respBody)
			isBase64 = true
		} else {
			responseBody = string(respBody)
		}

		cr := &CapturedRequest{
			ID:              id,
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

		if db != nil {
			err := db.Update(func(tx *bbolt.Tx) error {
				b := tx.Bucket([]byte(liveSessionBucketName))

				// Enforce request limit
				if b.Stats().KeyN >= maxCapturedRequests {
					c := b.Cursor()
					// Iterate and delete the oldest non-essential requests
					for k, v := c.First(); k != nil; k, v = c.Next() {
						var tempReq CapturedRequest
						if json.Unmarshal(v, &tempReq) == nil {
							if !tempReq.Bookmarked && tempReq.Note == "" {
								b.Delete(k)
								// Check if we are now under the limit
								if b.Stats().KeyN < maxCapturedRequests {
									break
								}
							}
						}
					}
				}

				// Save the new request
				jsonReq, _ := json.Marshal(cr)
				return b.Put([]byte(strconv.FormatInt(cr.ID, 10)), jsonReq)
			})

			if err != nil {
				log.Printf("Error capturing request to DB: %v", err)
				return
			}
		}

		wsBroadCast(cr)
	}()
}
