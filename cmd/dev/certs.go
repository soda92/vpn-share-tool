package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var certsCmd = &cobra.Command{
	Use:   "certs",
	Short: "Generate self-signed certificates for development",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGenCerts()
	},
}

func init() {
	rootCmd.AddCommand(certsCmd)
}

func runGenCerts() error {
	fmt.Println("Generating certificates...")
	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}
	
	certsDir := filepath.Join(rootDir, "certs")
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		return err
	}

	// 1. Generate CA
	caCert, caKey, err := generateCA()
	if err != nil {
		return err
	}
	
	if err := savePEM(filepath.Join(certsDir, "ca.crt"), "CERTIFICATE", caCert); err != nil {
		return err
	}
	if err := savePEM(filepath.Join(certsDir, "ca.key"), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caKey)); err != nil {
		return err
	}

	// 2. Generate Server Cert
	// We include common local IPs and "discovery-server" hostname
	ips := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("192.168.0.81"),
		net.ParseIP("192.168.1.81"),
	}
	serverCert, serverKey, err := generateServerCert(caCert, caKey, ips)
	if err != nil {
		return err
	}

	if err := savePEM(filepath.Join(certsDir, "server.crt"), "CERTIFICATE", serverCert); err != nil {
		return err
	}
	if err := savePEM(filepath.Join(certsDir, "server.key"), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverKey)); err != nil {
		return err
	}

	fmt.Println("✅ Certificates generated in ./certs/")
	return nil
}

func generateCA() ([]byte, *rsa.PrivateKey, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"VPN Share Tool CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour * 10), // 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	return derBytes, priv, err
}

func generateServerCert(caCert []byte, caKey *rsa.PrivateKey, ips []net.IP) ([]byte, *rsa.PrivateKey, error) {
	ca, err := x509.ParseCertificate(caCert)
	if err != nil {
		return nil, nil, err
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"VPN Share Tool Server"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: ips,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, ca, &priv.PublicKey, caKey)
	return derBytes, priv, err
}

func savePEM(path, blockType string, bytes []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: bytes})
}
