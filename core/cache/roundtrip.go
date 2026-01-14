package cache

import (
	"net/http"
	"runtime/trace"
)

// RoundTrip implements the http.RoundTripper interface.
func (t *CachingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, task := trace.NewTask(req.Context(), "ProxyRequest")
	defer task.End()
	req = req.WithContext(ctx)

	// Read the request body for capturing
	reqBody, err := t.readRequestBody(req)
	if err != nil {
		return nil, err
	}

	// 1. Intercept calendar.js
	if resp := t.handleCalendarJS(req, reqBody); resp != nil {
		return resp, nil
	}

	// 2. Intercept Captcha Image
	if resp, err := t.handleCaptchaImage(req, reqBody); resp != nil || err != nil {
		return resp, err
	}

	// 3. Intercept Captcha Solution Poll
	if resp := t.handleCaptchaPoll(req); resp != nil {
		return resp, nil
	}

	// 4. STATIC ASSETS: Cache Strategy (No Pipeline)
	if IsCacheable(req.URL.Path) {
		return t.handleStaticAsset(req, reqBody)
	}

	// 5. DYNAMIC/APP ASSETS: Pipeline Strategy (No Cache)
	return t.handleDynamicAsset(req, reqBody)
}
