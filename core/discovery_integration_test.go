package core

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/soda92/vpn-share-tool/core/debug"
	"github.com/soda92/vpn-share-tool/core/register"
)

func generateTestCert() (certPEM []byte, keyPEM []byte, err error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Discovery Server"},
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
		return nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certPEM, keyPEM, nil
}

func TestE2E_DiscoveryRegistration(t *testing.T) {
	// 0. Setup Temporary Storage
	tmpDir, err := os.MkdirTemp("", "vpn-share-discovery-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	debug.DebugStoragePath = tmpDir

	// 1. Setup Mock Discovery TLS Server
	certPEM, keyPEM, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("Failed to load test cert: %v", err)
	}
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Failed to start mock discovery server: %v", err)
	}
	defer listener.Close()
	
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	
	regReceived := make(chan string, 1)
	heartbeatReceived := make(chan bool, 1)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				scanner := bufio.NewScanner(c)
				for scanner.Scan() {
					msg := scanner.Text()
					parts := strings.Split(msg, " ")
					switch parts[0] {
					case "REGISTER":
						log.Printf("MockServer: Received REGISTER %v", parts)
						regReceived <- msg
						c.Write([]byte("OK 127.0.0.1\n"))
					case "HEARTBEAT":
						log.Printf("MockServer: Received HEARTBEAT %v", parts)
						heartbeatReceived <- true
						c.Write([]byte("OK\n"))
					}
				}
			}(conn)
		}
	}()

	// 2. Setup Client Registration Config
	var detectedIP string
	var discoveryURL string
	
	ipChan := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	cfg := register.Config{
		Ctx:              ctx,
		MyIP:             "127.0.0.1", 
		Version:          "test-v1",
		APIPort:          8888,
		DiscoverySrvPort: port,
		FallbackServerIPs: []string{"127.0.0.1"},
		// Use the generated certPEM as the root CA for this test
		RootCACert:       certPEM, 
		IPReadyChan:      ipChan,
		SetMyIP: func(ip string) {
			detectedIP = ip
		},
		UpdateDiscoveryURL: func(url string) {
			discoveryURL = url
		},
	}
	
	os.Remove(debug.DebugStoragePath + "/discovery_cache.json")

	// 3. Start Registration
	go register.Start(cfg)

	// 4. Verification
	// A. Wait for REGISTER command on server
	select {
	case msg := <-regReceived:
		if !strings.Contains(msg, "REGISTER 8888 test-v1") {
			t.Errorf("Unexpected REGISTER message: %s", msg)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for REGISTER")
	}

	// B. Wait for IPReady signal on client
	select {
	case ip := <-ipChan:
		if ip != "127.0.0.1" {
			t.Errorf("Expected IP 127.0.0.1, got %s", ip)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for IPReady signal")
	}

	// Now check if detectedIP was updated via the callback
	if detectedIP != "127.0.0.1" {
		t.Errorf("detectedIP not updated correctly via callback: %s", detectedIP)
	}

	// Verify discovery URL was updated
	if discoveryURL == "" {
		t.Error("discoveryURL was not updated")
	} else {
		log.Printf("Discovery URL updated to: %s", discoveryURL)
	}

	// C. Wait for HEARTBEAT
	select {
	case <-heartbeatReceived:
		log.Println("Heartbeat received successfully")
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for HEARTBEAT")
	}
}
