package cache

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/soda92/vpn-share-tool/core/debug"
)

func (t *CachingTransport) handleCaptchaImage(req *http.Request, reqBody []byte) (*http.Response, error) {
	if !strings.HasSuffix(req.URL.Path, "voCode") || t.Proxy == nil || !t.Proxy.GetEnableCaptcha() {
		return nil, nil
	}

	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	var respBody []byte
	if resp.Body != nil {
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		resp.Body.Close()
	}

	// Solve and Store (Async to avoid blocking image load)
	if len(respBody) > 0 && t.CaptchaProvider != nil {
		clientIP := getClientIP(req)
		// Clear old solution immediately to prevent JS from picking up stale data
		t.CaptchaProvider.Clear(clientIP)

		go func(data []byte, ip string) {
			solution := t.CaptchaProvider.Solve(data)
			if solution != "" {
				t.CaptchaProvider.Store(ip, solution)
			}
		}(respBody, clientIP)
	}

	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	debug.CaptureRequest(req, resp, reqBody, respBody)
	return resp, nil
}

func (t *CachingTransport) handleCaptchaPoll(req *http.Request) *http.Response {
	if !strings.HasSuffix(req.URL.Path, "/_proxy/captcha-solution") {
		return nil
	}

	if t.CaptchaProvider == nil {
		return nil
	}

	clientIP := getClientIP(req)
	solution := t.CaptchaProvider.Get(clientIP)

	if solution != "" {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(solution)),
			Request:    req,
		}
	}
	// Not ready yet
	return &http.Response{
		StatusCode: http.StatusNotFound, // JS will retry
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString("Not found")),
		Request:    req,
	}
}
