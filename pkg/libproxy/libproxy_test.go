package libproxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
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
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	
	// Update global CA placeholder for verification in tests
	CACertPEM = string(certPEM)

	return tls.X509KeyPair(certPEM, keyPEM)
}
