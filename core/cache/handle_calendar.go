package cache

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/resources"
)

func (t *CachingTransport) handleCalendarJS(req *http.Request, reqBody []byte) *http.Response {
	if strings.Contains(req.URL.Path, "calendar.js") {
		log.Printf("Intercepting calendar.js request: %s", req.URL.String())
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(resources.CalendarScript)),
			Request:    req,
		}
		resp.Header.Set("Content-Type", "application/javascript")
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(resources.CalendarScript)))

		debug.CaptureRequest(req, resp, reqBody, resources.CalendarScript)
		return resp
	}
	return nil
}
