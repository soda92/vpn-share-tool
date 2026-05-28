package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

// IsNewerVersion returns true if candidate is a newer version than current.
// Version format is expected to be like "v40d", "v41aa", etc.
func IsNewerVersion(current, candidate string) bool {
	// If current is "dev" or empty, we do not want to auto-update
	if current == "dev" || current == "" {
		return false
	}

	cntCur, sufCur, okCur := parseVersion(current)
	cntCan, sufCan, okCan := parseVersion(candidate)

	// If either is malformed, we err on the side of no update
	if !okCur || !okCan {
		return false
	}

	if cntCan != cntCur {
		return cntCan > cntCur
	}

	if len(sufCan) != len(sufCur) {
		return len(sufCan) > len(sufCur)
	}

	return sufCan > sufCur
}

// parseVersion parses strings like "v40d" or "40d" into counter and suffix.
func parseVersion(v string) (int, string, bool) {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		v = v[1:]
	}

	// Find where the integer ends and suffix begins
	idx := 0
	for idx < len(v) && v[idx] >= '0' && v[idx] <= '9' {
		idx++
	}

	if idx == 0 {
		return 0, "", false
	}

	cnt, err := strconv.Atoi(v[:idx])
	if err != nil {
		return 0, "", false
	}

	return cnt, v[idx:], true
}
