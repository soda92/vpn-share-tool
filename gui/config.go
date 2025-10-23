package gui

import (
	"encoding/json"
	"log"
	"os"

	"fyne.io/fyne/v2/widget"
)

// saveConfig saves the current list of original URLs to the config file.
func saveConfig() {
	proxiesLock.RLock()
	defer proxiesLock.RUnlock()

	var urls []string
	for _, p := range proxies {
		urls = append(urls, p.OriginalURL)
	}

	gconfig.OriginalURLs = urls
	data, err := json.MarshalIndent(gconfig, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal config to JSON: %v", err)
		return
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		log.Printf("Failed to write config file: %v", err)
	}
}

// loadConfig loads URLs from the config file and re-initializes the proxies.
func loadConfig(shareFunc func(string), statusLabel *widget.Label) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return // No config file yet
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Printf("Failed to read config file: %v", err)
		return
	}

	if err := json.Unmarshal(data, &gconfig); err != nil {
		log.Printf("Failed to unmarshal config JSON: %v", err)
		return
	}

	log.Printf("Loading %d URLs from config...", len(gconfig.OriginalURLs))
	for _, u := range gconfig.OriginalURLs {
		shareFunc(u)
	}
	statusLabel.SetText(l("serverRunning"))
}
