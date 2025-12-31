package core

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"log"
	"net/http"
	"time"

	"github.com/soda92/vpn-share-tool/core/proxy"
)

//go:embed ca.crt
var rootCACert []byte

const (
	discoverySrvPort = "45679"
)

var (
	// Fallback IPs if scanning fails.
	// 127.0.0.1 is prioritized for local testing.
	ServerIPs          = []string{"127.0.0.1", "192.168.0.81", "192.168.1.81"}
	APIPort            int
	MyIP               string
	DiscoveryServerURL string
	Version            string
)

func GetHTTPClient() *http.Client {
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(rootCACert); !ok {
		log.Println("Failed to append CA cert")
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
		Timeout: 30 * time.Second,
	}
}

// SetMyIP allows external packages (like mobile bridge) to set the client IP.
func SetMyIP(ip string) {
	MyIP = ip
	proxy.SetGlobalConfig(MyIP, APIPort, DiscoveryServerURL, GetHTTPClient)
	log.Printf("Device IP set to: %s", MyIP)
	// Trigger a signal? For now just logging.
}
