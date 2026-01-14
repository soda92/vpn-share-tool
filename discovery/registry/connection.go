package registry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

func HandleConnection(conn net.Conn) {
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
	for {
		// Set read deadline to detect dead clients (Heartbeat is every 5s)
		conn.SetReadDeadline(time.Now().Add(15 * time.Second))

		if !scanner.Scan() {
			break
		}

		// Reset write deadline for the response
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

		message := scanner.Text()
		parts := strings.Split(message, " ")
		command := parts[0]

		var response []byte
		shouldWrite := false

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

			mutex.Lock()
			instances[instanceAddress] = Instance{
				Address:  instanceAddress,
				Version:  version,
				LastSeen: time.Now(),
			}
			mutex.Unlock()

			log.Printf("Registered instance: %s (%s)", instanceAddress, version)
			response = []byte(fmt.Sprintf("OK %s\n", remoteAddr))
			shouldWrite = true

		case "LIST":
			mutex.Lock()
			activeInstances := make([]Instance, 0, len(instances))
			for _, instance := range instances {
				activeInstances = append(activeInstances, instance)
			}
			mutex.Unlock()

			data, err := json.Marshal(activeInstances)
			if err != nil {
				log.Printf("Failed to marshal instance list: %v", err)
				continue
			}
			response = append(data, '\n')
			shouldWrite = true

		case "HEARTBEAT":
			if len(parts) < 2 {
				log.Printf("Invalid HEARTBEAT command from %s", remoteAddr)
				break
			}
			apiPort := parts[1]
			instanceAddress = net.JoinHostPort(remoteAddr, apiPort)

			mutex.Lock()
			if existingInstance, ok := instances[instanceAddress]; ok {
				updatedInstance := existingInstance
				updatedInstance.LastSeen = time.Now()
				instances[instanceAddress] = updatedInstance
				response = []byte("OK\n")
			} else {
				log.Printf("Heartbeat from unregistered instance: %s", instanceAddress)
				response = []byte("ERR_NOT_REGISTERED\n")
			}
			mutex.Unlock()
			shouldWrite = true

		default:
			log.Printf("Unknown command from %s: %s", remoteAddr, command)
		}

		if shouldWrite {
			if _, err := conn.Write(response); err != nil {
				log.Printf("Error writing to %s: %v", remoteAddr, err)
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from connection from %s: %v", remoteAddr, err)
	}
}
