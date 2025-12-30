package core

import (
	"context"
	"net/http"
	"net/http/httputil"
	"sync"
)

type contextKey string

const originalHostKey contextKey = "originalHost"

type SharedProxy struct {
	OriginalURL   string                 `json:"original_url"`
	RemotePort    int                    `json:"remote_port"`
	Path          string                 `json:"path"`
	Handler       *httputil.ReverseProxy `json:"-"`
	Server        *http.Server           `json:"-"`
	EnableDebug   bool                   `json:"enable_debug"`
	EnableCaptcha bool                   `json:"enable_captcha"`
	RequestRate   float64                `json:"request_rate"`
	TotalRequests int64                  `json:"total_requests"`
	mu            sync.RWMutex
	reqCounter    int64              // Atomic counter for current second
	ctx           context.Context    // Context for lifecycle management
	cancel        context.CancelFunc // Function to cancel the context
}

func (p *SharedProxy) SetEnableDebug(enable bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.EnableDebug = enable
}

func (p *SharedProxy) GetEnableDebug() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.EnableDebug
}

func (p *SharedProxy) SetEnableCaptcha(enable bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.EnableCaptcha = enable
}

func (p *SharedProxy) GetEnableCaptcha() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.EnableCaptcha
}
