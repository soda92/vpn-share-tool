package registry

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

type Instance struct {
	Address  string    `json:"address"`
	Version  string    `json:"version"`
	LastSeen time.Time `json:"last_seen"`
}

var (
	cleanupInterval = 1 * time.Minute
	staleTimeout    = 5 * time.Minute

	instances = make(map[string]Instance)
	mutex     = &sync.Mutex{}
)

func GetActiveInstances() []Instance {
	mutex.Lock()
	defer mutex.Unlock()

	activeInstances := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		activeInstances = append(activeInstances, instance)
	}
	return activeInstances
}

func StartCleanupTask() {
	for {
		time.Sleep(cleanupInterval)
		mutex.Lock()
		// log.Println("Running cleanup of stale instances...")
		for addr, instance := range instances {
			if time.Since(instance.LastSeen) > staleTimeout {
				log.Printf("Removing stale instance: %s", addr)
				delete(instances, addr)
			}
		}
		mutex.Unlock()
	}
}
