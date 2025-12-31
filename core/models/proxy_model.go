package models

import (
	"context"
	"net/http"
	"net/http/httputil"
	"sync"
)

type contextKey string

const OriginalHostKey contextKey = "originalHost"

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
	Mu            sync.RWMutex
	ReqCounter    int64              // Atomic counter for current second
	Ctx           context.Context    // Context for lifecycle management
	Cancel        context.CancelFunc // Function to cancel the context
}

func (p *SharedProxy) SetEnableDebug(enable bool) {
	p.Mu.Lock()
	defer p.Mu.Unlock()
	p.EnableDebug = enable
}

func (p *SharedProxy) GetEnableDebug() bool {
	p.Mu.RLock()
	defer p.Mu.RUnlock()
	return p.EnableDebug
}

func (p *SharedProxy) SetEnableCaptcha(enable bool) {
	p.Mu.Lock()
	defer p.Mu.Unlock()
	p.EnableCaptcha = enable
}

func (p *SharedProxy) GetEnableCaptcha() bool {
	p.Mu.RLock()
	defer p.Mu.RUnlock()
	return p.EnableCaptcha
}
