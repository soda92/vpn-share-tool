package transport

import (
	"crypto/tls"
	"log"
	"net"
	"github.com/soda92/vpn-share-tool/discovery/resources"
	"github.com/soda92/vpn-share-tool/discovery/registry"
)

const listenPort = "45679"

func StartTCPServer() {
	log.Printf("Starting discovery TCP server on port %s", listenPort)

	var listener net.Listener
	var err error

	cer, errLoad := tls.X509KeyPair(resources.ServerCert, resources.ServerKey)
	if errLoad == nil {
		log.Printf("Embedded TLS certificates found. Starting Secure TCP Server.")
		config := &tls.Config{Certificates: []tls.Certificate{cer}}
		listener, err = tls.Listen("tcp", ":"+listenPort, config)
	} else {
		log.Printf("Failed to load embedded certs: %v. Starting Insecure TCP Server.", errLoad)
		listener, err = net.Listen("tcp", ":"+listenPort)
	}

	if err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}
	defer listener.Close()

	// Periodically clean up stale instances
	go registry.StartCleanupTask()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go registry.HandleConnection(conn)
	}
}
