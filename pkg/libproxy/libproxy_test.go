package libproxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

// Helper to reset globals
func resetGlobals() {
	DiscoveryServerHosts = []string{"192.168.0.81", "192.168.1.81"}
	DiscoveryServerPort = 45679
}

func TestCache(t *testing.T) {
	// Setup temp home for cache file
	tmpDir, err := os.MkdirTemp("", "libproxy_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	os.Setenv("HOME", tmpDir)
	os.Setenv("APPDATA", tmpDir)
	// For Windows, UserConfigDir often relies on APPDATA. 
	// For Linux, it relies on XDG_CONFIG_HOME or HOME/.config.
	os.Setenv("XDG_CONFIG_HOME", tmpDir) 

	target := "http://example.com"
	proxy := "http://proxy.local:8080"

	saveToCache(target, proxy)

	cache := loadCache()
	if val, ok := cache[target]; !ok || val != proxy {
		t.Errorf("Cache failed. Expected %s, got %s", proxy, val)
	}
}

func TestScanSubnet(t *testing.T) {
	// Pick a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("Skipping scan test, cannot listen")
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	
	// Scan localhost
	hosts := scanSubnet("127.0.0.1", port)
	
	found := false
	for _, h := range hosts {
		if h == "127.0.0.1" {
			found = true
			break
		}
	}
	if !found {
		t.Logf("Scan failed to find 127.0.0.1 on port %d. Hosts found: %v", port, hosts)
	}
}

// Helper to generate a self-signed cert for testing
func generateTestCert() (tls.Certificate, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Co"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return tls.X509KeyPair(certPEM, keyPEM)
}

func TestDiscoverProxy_Flow(t *testing.T) {
	defer resetGlobals()

	// 1. Start Mock API Server (Simulate a vpn-share-tool instance)
	mux := http.NewServeMux()
	apiServer := httptest.NewServer(mux)
	defer apiServer.Close()
	
	apiURL, _ := url.Parse(apiServer.URL)
	apiAddr := apiURL.Host // ip:port

	// Mock /services endpoint
	mux.HandleFunc("/services", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Service{
			{OriginalURL: "http://cached.com", SharedURL: "http://proxy.local/cached"},
		})
	})

	// Mock /can-reach endpoint
	mux.HandleFunc("/can-reach", func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query().Get("url")
		reachable := u == "http://reachable.com"
		json.NewEncoder(w).Encode(ReachResponse{Reachable: reachable})
	})

	// Mock /proxies endpoint (Create)
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		var req CreateProxyRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.URL == "http://reachable.com" {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(Service{
				OriginalURL: req.URL,
				SharedURL:   "http://proxy.local/created",
			})
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	// 2. Start Mock Discovery Server (TLS)
	cer, err := generateTestCert()
	if err != nil {
		t.Fatal(err)
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}
	
	mdsListener, err := tls.Listen("tcp", "127.0.0.1:0", config)
	if err != nil {
		t.Fatal(err)
	}
	defer mdsListener.Close()
	
	mdsPort := mdsListener.Addr().(*net.TCPAddr).Port
	DiscoveryServerPort = mdsPort
	DiscoveryServerHosts = []string{"127.0.0.1"}

	// Handle MDS connections
	go func() {
		for {
			conn, err := mdsListener.Accept()
			if err != nil { return }
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				n, _ := c.Read(buf)
				if n > 0 && string(buf[:n]) == "LIST\n" {
					resp := []InstanceResponse{{Address: apiAddr}}
					data, _ := json.Marshal(resp)
					c.Write(data)
				}
			}(conn)
		}
	}()

	// 3. Run Tests
	
	// Case A: Existing Proxy (mocked via /services)
	// Target: http://cached.com -> should return http://proxy.local/cached
	p, err := DiscoverProxy("http://cached.com", 1*time.Second, false)
	if err != nil {
		t.Errorf("Case A failed: unexpected error %v", err)
	} else if p != "http://proxy.local/cached" {
		t.Errorf("Case A failed: expected http://proxy.local/cached, got %s", p)
	}

	// Case B: Create Proxy (mocked via /can-reach and /proxies)
	// Target: http://reachable.com -> should return http://proxy.local/created
	p, err = DiscoverProxy("http://reachable.com", 1*time.Second, false)
	if err != nil {
		t.Errorf("Case B failed: unexpected error %v", err)
	} else if p != "http://proxy.local/created" {
		t.Errorf("Case B failed: expected http://proxy.local/created, got %s", p)
	}
	
	// Case C: Unreachable
	p, err = DiscoverProxy("http://unreachable.com", 1*time.Second, false)
	if err == nil {
		t.Errorf("Case C failed: expected error for unreachable url, got %s", p)
	}
}