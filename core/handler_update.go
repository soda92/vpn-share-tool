package core

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func handleTriggerUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go func() {
		updated, err := TriggerUpdate()
		if err != nil {
			log.Printf("Remote trigger update failed: %v", err)
		} else if updated {
			log.Printf("Remote triggered update success. Exiting.")
			// The process will exit in TriggerUpdate usually, but if not:
			// os.Exit(0)
		}
	}()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Update triggered"))
}

type UpdateInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

func CheckForUpdates() (*UpdateInfo, error) {
	if DiscoveryServerURL == "" {
		return nil, fmt.Errorf("discovery server not connected")
	}

	client := GetHTTPClient()
	resp, err := client.Get(DiscoveryServerURL + "/latest-version")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	var info UpdateInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	return &info, nil
}
