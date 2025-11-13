package main

import (
	"log"
	"net"
)

const listenPort = "45679"

func startTCPServer() {
	log.Printf("Starting discovery TCP server on port %s", listenPort)
	listener, err := net.Listen("tcp", ":"+listenPort)
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
