package main

import (
	"crypto/tls"
	"log"
	"net"
)

const listenPort = "45679"

func startTCPServer() {
	log.Printf("Starting discovery TCP server on port %s", listenPort)

	var listener net.Listener
	var err error

	cer, errLoad := tls.X509KeyPair(serverCert, serverKey)
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
	go cleanupStaleInstances()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}
