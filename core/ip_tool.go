package core

import (
	"net/http"
	"strings"
)

func getClientIP(req *http.Request) string {
	// Get Client IP
	clientIP := req.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = req.RemoteAddr
	}
	if strings.Contains(clientIP, ",") {
		clientIP = strings.Split(clientIP, ",")[0]
	}
	clientIP = strings.TrimSpace(clientIP)

	return clientIP
}
