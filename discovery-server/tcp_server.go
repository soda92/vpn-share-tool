package main

import (
	"crypto/tls"
	"log"
	"net"
	"os"
)

const listenPort = "45679"

func startTCPServer() {
	log.Printf("Starting discovery TCP server on port %s", listenPort)
	
	var listener net.Listener
	var err error

	certFile := "certs/server.crt"
	keyFile := "certs/server.key"

	if _, errStat := os.Stat(certFile); errStat == nil {
		if _, errStat := os.Stat(keyFile); errStat == nil {
			log.Printf("TLS certificates found. Starting Secure TCP Server.")
			cer, errLoad := tls.LoadX509KeyPair(certFile, keyFile)
			if errLoad != nil {
				log.Fatalf("Failed to load keypair: %v", errLoad)
			}
			config := &tls.Config{Certificates: []tls.Certificate{cer}}
			listener, err = tls.Listen("tcp", ":"+listenPort, config)
		}
	}

	if listener == nil {
		log.Printf("No certificates found. Starting Insecure TCP Server.")
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
