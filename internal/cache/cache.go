package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const cacheFileName = ".sanshain-cache.json"

type RequireEntry struct {
	Etag        string `json:"etag"`
	LastFetched string `json:"last_fetched"`
}

type CacheState struct {
	Requires map[string]*RequireEntry `json:"requires"`
}

type SanshainCache struct {
	cacheFile string
	State     CacheState
}

func NewSanshainCache(cacheDir string) *SanshainCache {
	c := &SanshainCache{
		cacheFile: filepath.Join(cacheDir, cacheFileName),
	}
	c.load()
	return c
}

func (c *SanshainCache) load() {
	c.State = CacheState{
		Requires: make(map[string]*RequireEntry),
	}
	data, err := os.ReadFile(c.cacheFile)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &c.State)
	if c.State.Requires == nil {
		c.State.Requires = make(map[string]*RequireEntry)
	}
}

func (c *SanshainCache) Save() error {
	dir := filepath.Dir(c.cacheFile)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c.State, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.cacheFile, data, 0600)
}

func (c *SanshainCache) GetRequireEntry(key string) *RequireEntry {
	return c.State.Requires[key]
}

func (c *SanshainCache) UpdateRequireEntry(key, etag string) {
	c.State.Requires[key] = &RequireEntry{
		Etag:        etag,
		LastFetched: time.Now().UTC().Format(time.RFC3339),
	}
}

// RequireKey keys a single-endpoint require. A requested format is part of
// the key, so two formats of the same endpoint never share an ETag.
func RequireKey(producerName, version, method, path, format string) string {
	return withFormat(fmt.Sprintf("%s|%s|%s|%s", producerName, version, method, path), format)
}

// RequireBundleKey keys a bundle require, including a requested format.
func RequireBundleKey(producerName, version, format string) string {
	return withFormat(fmt.Sprintf("%s|%s|bundle", producerName, version), format)
}

func withFormat(key, format string) string {
	if format == "" {
		return key
	}
	return key + "|format=" + format
}
