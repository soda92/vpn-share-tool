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
	requestIDLock.Lock()
	nextRequestID++

	isBase64 := false
	var responseBody string

	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "image/") || !utf8.Valid(respBody) {
		responseBody = base64.StdEncoding.EncodeToString(respBody)
		isBase64 = true
	} else {
		responseBody = string(respBody)
	}

	cr := &CapturedRequest{
		ID:              nextRequestID,
		Timestamp:       time.Now(),
		Method:          req.Method,
		URL:             req.URL.String(),
		RequestHeaders:  req.Header,
		RequestBody:     string(reqBody),
		ResponseStatus:  resp.StatusCode,
		ResponseHeaders: resp.Header,
		ResponseBody:    responseBody,
		IsBase64:        isBase64,
	}
	requestIDLock.Unlock()

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
}
