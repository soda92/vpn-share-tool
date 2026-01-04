package store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

const storageFilePath = "tagged_urls.json"

func LoadTaggedURLs() {
	taggedURLsMutex.Lock()
	defer taggedURLsMutex.Unlock()

	data, err := os.ReadFile(storageFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No tagged URLs file found, starting fresh.")
			return
		}
		log.Printf("Error reading tagged URLs file: %v", err)
		return
	}

	if err := json.Unmarshal(data, &taggedURLs); err != nil {
		log.Printf("Error unmarshaling tagged URLs: %v", err)
	}
	log.Printf("Loaded %d tagged URLs from %s", len(taggedURLs), storageFilePath)
}

func saveTaggedURLs() error {
	taggedURLsMutex.Lock()
	defer taggedURLsMutex.Unlock()

	data, err := json.MarshalIndent(taggedURLs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tagged URLs: %w", err)
	}

	if err := os.WriteFile(storageFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tagged URLs file: %w", err)
	}
	return nil
}
