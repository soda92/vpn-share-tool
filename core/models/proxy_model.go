package models

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

type contextKey string

const OriginalHostKey contextKey = "originalHost"

type PipelineServices struct {
	CreateProxy func(url string, port int) (*SharedProxy, error)
	MyIP        string
	APIPort     int
}

type ProcessingContext struct {
	ReqURL     *url.URL
	ReqContext context.Context
	RespHeader http.Header
	Proxy      *SharedProxy
	Services   PipelineServices
}

type ProxySettings struct {
	EnableContentMod bool `json:"enable_content_mod"`
	EnableUrlRewrite bool `json:"enable_url_rewrite"`
}

type SharedProxy struct {
	OriginalURL   string                 `json:"original_url"`
	RemotePort    int                    `json:"remote_port"`
	Path          string                 `json:"path"`
	Handler       *httputil.ReverseProxy `json:"-"`
	Server        *http.Server           `json:"-"`
	Settings      ProxySettings          `json:"settings"`
	ActiveSystems []string               `json:"active_systems"`
	RequestRate   float64                `json:"request_rate"`
	TotalRequests int64                  `json:"total_requests"`
	Mu            sync.RWMutex           `json:"-"`
	ReqCounter    int64                  `json:"-"` // Atomic counter for current second
	Ctx           context.Context        `json:"-"` // Context for lifecycle management
	Cancel        context.CancelFunc     `json:"-"` // Function to cancel the context
}
