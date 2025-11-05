package core

import (
	"time"
)

// Custom metadata to be embedded in the HAR format.
type VpnShareToolMetadata struct {
	Bookmarked bool   `json:"bookmarked,omitempty"`
	Note       string `json:"note,omitempty"`
}

// HAR is the top-level structure for a HAR file.

type HAR struct {
	Log Log `json:"log"`
}

type Log struct {
	Version string  `json:"version"`
	Creator Creator `json:"creator"`
	Entries []Entry `json:"entries"`
}

type Creator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Entry struct {
	StartedDateTime time.Time              `json:"startedDateTime"`
	Time            int64                  `json:"time"`
	Request         Request                `json:"request"`
	Response        Response               `json:"response"`
	Cache           map[string]interface{} `json:"cache"`
	Timings         Timings                `json:"timings"`
	// Custom field for VPN Share Tool specific metadata
	VpnShareToolMetadata *VpnShareToolMetadata `json:"_vpnShareToolMetadata,omitempty"`
}

type Request struct {
	Method      string      `json:"method"`
	URL         string      `json:"url"`
	HTTPVersion string      `json:"httpVersion"`
	Cookies     []Cookie    `json:"cookies"`
	Headers     []NameValuePair `json:"headers"`
	QueryString []NameValuePair `json:"queryString"`
	PostData    *PostData   `json:"postData,omitempty"`
	HeadersSize int64       `json:"headersSize"`
	BodySize    int64       `json:"bodySize"`
}

type Response struct {
	Status      int         `json:"status"`
	StatusText  string      `json:"statusText"`
	HTTPVersion string      `json:"httpVersion"`
	Cookies     []Cookie    `json:"cookies"`
	Headers     []NameValuePair `json:"headers"`
	Content     Content     `json:"content"`
	RedirectURL string      `json:"redirectURL"`
	HeadersSize int64       `json:"headersSize"`
	BodySize    int64       `json:"bodySize"`
}

type NameValuePair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Path     string    `json:"path"`
	Domain   string    `json:"domain"`
	Expires  time.Time `json:"expires"`
	HTTPOnly bool      `json:"httpOnly"`
	Secure   bool      `json:"secure"`
}

type PostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type Content struct {
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:from_json,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

type Timings struct {
	Send    int64 `json:"send"`
	Wait    int64 `json:"wait"`
	Receive int64 `json:"receive"`
}
