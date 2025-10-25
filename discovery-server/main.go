package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	listenPort = "45679"
)

type Instance struct {
	Address  string    `json:"address"`
	LastSeen time.Time `json:"last_seen"`
}

var (
	instances       = make(map[string]Instance)
	mutex           = &sync.Mutex{}
	cleanupInterval = 1 * time.Minute
	staleTimeout    = 5 * time.Minute
)

func main() {
	log.Printf("Starting discovery server on port %s", listenPort)
	listener, err := net.Listen("tcp", ":"+listenPort)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
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

func handleConnection(conn net.Conn) {
	defer conn.Close()
	remoteAddr := conn.RemoteAddr().(*net.TCPAddr).IP.String()
	log.Printf("Accepted connection from %s", remoteAddr)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		message := scanner.Text()
		parts := strings.Split(message, " ")
		command := parts[0]

		mutex.Lock()
		switch command {
		case "REGISTER":
			if len(parts) < 2 {
				log.Printf("Invalid REGISTER command from %s", remoteAddr)
				break
			}
			apiPort := parts[1]
			instanceAddress := net.JoinHostPort(remoteAddr, apiPort)
			instances[instanceAddress] = Instance{
				Address:  instanceAddress,
				LastSeen: time.Now(),
			}
			log.Printf("Registered instance: %s", instanceAddress)
			response := fmt.Sprintf("OK %s\n", remoteAddr)
			if _, err := conn.Write([]byte(response)); err != nil {
				log.Printf("Error writing to %s: %v", remoteAddr, err)
				mutex.Unlock()
				return
			}

		case "LIST":
			var activeInstances []Instance
			for _, instance := range instances {
				activeInstances = append(activeInstances, instance)
			}
			mutex.Unlock() // Unlock early

			data, err := json.Marshal(activeInstances)
			if err != nil {
				log.Printf("Failed to marshal instance list: %v", err)
				continue // Skip to next loop iteration
			}
			if _, err := conn.Write(data); err != nil {
				log.Printf("Error writing to %s: %v", remoteAddr, err)
				return // Exit function
			}
			if _, err := conn.Write([]byte("\n")); err != nil {
				log.Printf("Error writing to %s: %v", remoteAddr, err)
				return // Exit function
			}
			continue // Continue to next loop iteration to avoid double-unlock

		case "HEARTBEAT":
			if len(parts) < 2 {
				log.Printf("Invalid HEARTBEAT command from %s", remoteAddr)
				break
			}
			apiPort := parts[1]
			instanceAddress := net.JoinHostPort(remoteAddr, apiPort)
			if _, ok := instances[instanceAddress]; ok {
				instances[instanceAddress] = Instance{
					Address:  instanceAddress,
					LastSeen: time.Now(),
				}
				log.Printf("Heartbeat from: %s", instanceAddress)
				if _, err := conn.Write([]byte("OK\n")); err != nil {
					log.Printf("Error writing to %s: %v", remoteAddr, err)
					mutex.Unlock()
					return
				}
			} else {
				log.Printf("Heartbeat from unregistered instance: %s", instanceAddress)
				if _, err := conn.Write([]byte("ERR_NOT_REGISTERED\n")); err != nil {
					log.Printf("Error writing to %s: %v", remoteAddr, err)
					mutex.Unlock()
					return
				}
			}

		default:
			log.Printf("Unknown command from %s: %s", remoteAddr, command)
		}
		mutex.Unlock()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from connection from %s: %v", remoteAddr, err)
	}
	log.Printf("Connection from %s closed", remoteAddr)
}

func cleanupStaleInstances() {
	for {
		time.Sleep(cleanupInterval)
		mutex.Lock()
		log.Println("Running cleanup of stale instances...")
		for addr, instance := range instances {
			if time.Since(instance.LastSeen) > staleTimeout {
				log.Printf("Removing stale instance: %s", addr)
				delete(instances, addr)
			}
		}
		mutex.Unlock()
	}
}
