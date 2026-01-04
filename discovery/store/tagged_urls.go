package store

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type TaggedURL struct {
	ID        string    `json:"id"`
	Tag       string    `json:"tag"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	taggedURLs      = make(map[string]TaggedURL)
	taggedURLsMutex = &sync.Mutex{}
)

func GetTaggedURLs() []TaggedURL {
	taggedURLsMutex.Lock()
	defer taggedURLsMutex.Unlock()
	urls := make([]TaggedURL, 0, len(taggedURLs))
	for _, u := range taggedURLs {
		urls = append(urls, u)
	}
	return urls
}

func AddTaggedURL(tag, urlStr string) (*TaggedURL, error) {
	newURL := TaggedURL{
		ID:        uuid.New().String(),
		Tag:       tag,
		URL:       urlStr,
		CreatedAt: time.Now(),
	}

	taggedURLsMutex.Lock()
	taggedURLs[newURL.ID] = newURL
	taggedURLsMutex.Unlock()

	if err := saveTaggedURLs(); err != nil {
		return nil, err
	}
	return &newURL, nil
}

func UpdateTaggedURL(id, tag string) error {
	taggedURLsMutex.Lock()
	defer taggedURLsMutex.Unlock()
	
	urlToUpdate, ok := taggedURLs[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	urlToUpdate.Tag = tag
	taggedURLs[id] = urlToUpdate

	return saveTaggedURLs()
}

func DeleteTaggedURL(id string) error {
	taggedURLsMutex.Lock()
	defer taggedURLsMutex.Unlock()
	
	_, ok := taggedURLs[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	delete(taggedURLs, id)

	return saveTaggedURLs()
}
