package core

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type UpdateInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	Sha256  string `json:"sha256"`
}

func CheckForUpdates() (*UpdateInfo, error) {
	if DiscoveryServerURL == "" {
		return nil, fmt.Errorf("discovery server not connected")
	}

	client := GetHTTPClient()
	resp, err := client.Get(DiscoveryServerURL + "/latest-app-version?format=zip")
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
