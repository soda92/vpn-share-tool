
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

const (
	discoveryPort     = 45678
	discoveryReqPrefix = "DISCOVER_REQ:"
	discoveryRespPrefix = "DISCOVER_RESP:"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <url_to_discover>", os.Args[0])
	}
	targetURL := os.Args[1]

	// Get all broadcast addresses
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatalf("Failed to get interface addresses: %v", err)
	}

	var broadcastAddrs []*net.UDPAddr
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.To4()
				mask := ipnet.Mask
				broadcast := net.IP(make([]byte, 4))
				for i := range ip {
					broadcast[i] = ip[i] | ^mask[i]
				}
				broadcastAddrs = append(broadcastAddrs, &net.UDPAddr{
					IP:   broadcast,
					Port: discoveryPort,
				})
			}
		}
	}

	if len(broadcastAddrs) == 0 {
		log.Fatalf("No suitable network interface found for broadcast.")
	}

	// Listen for a response
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: discoveryPort})
	if err != nil {
		log.Fatalf("Failed to listen for UDP response: %v", err)
	}
	defer conn.Close()

	// Send broadcast messages
	message := []byte(discoveryReqPrefix + targetURL)
	for _, baddr := range broadcastAddrs {
		_, err := conn.WriteToUDP(message, baddr)
		if err != nil {
			log.Printf("Failed to send broadcast to %s: %v", baddr, err)
		}
	}

	// Wait for a response with a timeout
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		// This is expected on timeout, so exit gracefully
		return
	}

	response := string(buf[:n])
	if strings.HasPrefix(response, discoveryRespPrefix) {
		proxyURL := strings.TrimPrefix(response, discoveryRespPrefix)
		fmt.Print(proxyURL) // Print the result to stdout
	}
}
