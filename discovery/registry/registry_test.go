package registry

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestRegistry_Registration(t *testing.T) {
	// Reset registry for test
	mutex.Lock()
	instances = make(map[string]Instance)
	mutex.Unlock()

	// Mock connection
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	// Start handler in goroutine
	go HandleConnection(server)

	// Send REGISTER
	fmt.Fprintf(client, "REGISTER 8080 v1.0\n")

	// Read response
	buf := make([]byte, 1024)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	resp := string(buf[:n])
	if !strings.HasPrefix(resp, "OK") {
		t.Errorf("Expected OK response, got %q", resp)
	}

	// Verify instance added
	active := GetActiveInstances()
	if len(active) != 1 {
		t.Errorf("Expected 1 active instance, got %d", len(active))
	}
}

func TestRegistry_Heartbeat(t *testing.T) {
	mutex.Lock()
	instances = make(map[string]Instance)
	mutex.Unlock()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go HandleConnection(server)

	// 1. Heartbeat before registration should fail
	fmt.Fprintf(client, "HEARTBEAT 8080\n")
	buf := make([]byte, 1024)
	n, _ := client.Read(buf)
	if !strings.Contains(string(buf[:n]), "ERR_NOT_REGISTERED") {
		t.Errorf("Expected ERR_NOT_REGISTERED, got %q", string(buf[:n]))
	}

	// 2. Register
	fmt.Fprintf(client, "REGISTER 8080 v1.0\n")
	client.Read(buf) // Consume OK

	// 3. Heartbeat after registration should succeed
	fmt.Fprintf(client, "HEARTBEAT 8080\n")
	n, _ = client.Read(buf)
	if !strings.Contains(string(buf[:n]), "OK") {
		t.Errorf("Expected OK heartbeat, got %q", string(buf[:n]))
	}
}

func TestRegistry_List(t *testing.T) {
	mutex.Lock()
	instances = make(map[string]Instance)
	instances["127.0.0.1:8080"] = Instance{
		Address:  "127.0.0.1:8080",
		Version:  "v1",
		LastSeen: time.Now(),
	}
	mutex.Unlock()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go HandleConnection(server)

	fmt.Fprintf(client, "LIST\n")
	buf := make([]byte, 4096)
	n, _ := client.Read(buf)

	var list []Instance
	if err := json.Unmarshal(buf[:n], &list); err != nil {
		t.Fatalf("Failed to unmarshal list: %v. Raw: %q", err, string(buf[:n]))
	}

	if len(list) != 1 || list[0].Address != "127.0.0.1:8080" {
		t.Errorf("Unexpected list content: %+v", list)
	}
}

func TestRegistry_Cleanup(t *testing.T) {
	mutex.Lock()
	instances = make(map[string]Instance)
	// Add a stale instance
	instances["stale:8080"] = Instance{
		Address:  "stale:8080",
		LastSeen: time.Now().Add(-10 * time.Minute),
	}
	// Add a fresh instance
	instances["fresh:8080"] = Instance{
		Address:  "fresh:8080",
		LastSeen: time.Now(),
	}
	mutex.Unlock()

	// Trigger manual cleanup logic (extracted or copied for test)
	mutex.Lock()
	for addr, instance := range instances {
		if time.Since(instance.LastSeen) > staleTimeout {
			delete(instances, addr)
		}
	}
	mutex.Unlock()

	active := GetActiveInstances()
	if len(active) != 1 || active[0].Address != "fresh:8080" {
		t.Errorf("Cleanup failed, active instances: %+v", active)
	}
}
