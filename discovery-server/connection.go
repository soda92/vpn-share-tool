package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

func handleConnection(conn net.Conn) {
	var instanceAddress string
	remoteAddr := conn.RemoteAddr().(*net.TCPAddr).IP.String()

	defer func() {
		conn.Close()
		if instanceAddress != "" {
			mutex.Lock()
			delete(instances, instanceAddress)
			mutex.Unlock()
			log.Printf("Unregistered instance due to connection close: %s", instanceAddress)
		}
		log.Printf("Connection from %s closed", remoteAddr)
	}()

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
				log.Printf("Invalid REGISTER command from %s: %v", remoteAddr, parts)
				break
			}
			apiPort := parts[1]
			version := "unknown"
			if len(parts) >= 3 {
				version = parts[2]
			}

			instanceAddress = net.JoinHostPort(remoteAddr, apiPort)
			instances[instanceAddress] = Instance{
				Address:  instanceAddress,
				Version:  version,
				LastSeen: time.Now(),
			}
			log.Printf("Registered instance: %s (v%s)", instanceAddress, version)
			response := fmt.Sprintf("OK %s\n", remoteAddr)
			if _, err := conn.Write([]byte(response)); err != nil {
				log.Printf("Error writing to %s: %v", remoteAddr, err)
				mutex.Unlock()
				return
			}

		case "LIST":
			activeInstances := make([]Instance, 0, len(instances))
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
			instanceAddress = net.JoinHostPort(remoteAddr, apiPort)
			if _, ok := instances[instanceAddress]; ok {
				instances[instanceAddress] = Instance{
					Address:  instanceAddress,
					LastSeen: time.Now(),
				}
				// log.Printf("Heartbeat from: %s", instanceAddress)
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
}
