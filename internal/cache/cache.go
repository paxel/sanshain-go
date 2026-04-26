package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const cacheFileName = ".sanshain-cache.json"

type ProvideEntry struct {
	ContentHash  string `json:"content_hash"`
	Version      int    `json:"version"`
	LastProvided string `json:"last_provided"`
}

type RequireEntry struct {
	Etag        string `json:"etag"`
	LastFetched string `json:"last_fetched"`
}

type CacheState struct {
	Provides map[string]*ProvideEntry `json:"provides"`
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
		Provides: make(map[string]*ProvideEntry),
		Requires: make(map[string]*RequireEntry),
	}
	data, err := os.ReadFile(c.cacheFile)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &c.State)
	if c.State.Provides == nil {
		c.State.Provides = make(map[string]*ProvideEntry)
	}
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

func (c *SanshainCache) GetProvideEntry(key string) *ProvideEntry {
	return c.State.Provides[key]
}

func (c *SanshainCache) UpdateProvideEntry(key, contentHash string, version int) {
	c.State.Provides[key] = &ProvideEntry{
		ContentHash:  contentHash,
		Version:      version,
		LastProvided: time.Now().UTC().Format(time.RFC3339),
	}
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

func ComputeHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("sha256:%x", h)
}

func RequireKey(serviceName, branch, method, path string) string {
	return fmt.Sprintf("%s|%s|%s|%s", serviceName, branch, method, path)
}

func RequireBundleKey(serviceName, branch string) string {
	return fmt.Sprintf("%s|%s|bundle", serviceName, branch)
}
