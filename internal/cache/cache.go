// Package cache keeps the require state file: the ETag of every require, so
// that an unchanged endpoint is not downloaded again.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const cacheFileName = ".sanshain-cache.json"

// RequireEntry is the state of one require: its ETag and when it was fetched.
type RequireEntry struct {
	Etag        string `json:"etag"`
	LastFetched string `json:"last_fetched"`
}

// State is the content of the state file.
type State struct {
	Requires map[string]*RequireEntry `json:"requires"`
}

// SanshainCache is the state file and what was read from it.
type SanshainCache struct {
	cacheFile string
	State     State
}

// NewSanshainCache reads the state file in cacheDir. A file that is missing
// or cannot be read gives an empty state.
func NewSanshainCache(cacheDir string) *SanshainCache {
	c := &SanshainCache{
		cacheFile: filepath.Join(cacheDir, cacheFileName),
	}
	c.load()
	return c
}

func (c *SanshainCache) load() {
	data, err := os.ReadFile(c.cacheFile)
	if err != nil {
		c.State = State{Requires: make(map[string]*RequireEntry)}
		return
	}
	c.State = decodeState(data)
}

// decodeState reads the content of the state file. What cannot be read counts
// as absent, so the state always has its map of requires.
func decodeState(data []byte) State {
	state := State{Requires: make(map[string]*RequireEntry)}
	_ = json.Unmarshal(data, &state)
	if state.Requires == nil {
		state.Requires = make(map[string]*RequireEntry)
	}
	return state
}

// Save writes the state file, creating its directory.
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

// GetRequireEntry returns the state of the require with this key, nil when
// there is none.
func (c *SanshainCache) GetRequireEntry(key string) *RequireEntry {
	return c.State.Requires[key]
}

// UpdateRequireEntry records etag for the require with this key, fetched now.
func (c *SanshainCache) UpdateRequireEntry(key, etag string) {
	c.State.Requires[key] = &RequireEntry{
		Etag:        etag,
		LastFetched: time.Now().UTC().Format(time.RFC3339),
	}
}

// RequireKey keys a single-endpoint require. A requested format and the
// output directory are part of the key, so two formats of the same endpoint,
// or two directories it is written to, never share an ETag.
func RequireKey(producerName, version, method, path, format, outputDir string) string {
	return withOutput(withFormat(fmt.Sprintf("%s|%s|%s|%s", producerName, version, method, path), format), outputDir)
}

// RequireBundleKey keys a bundle require, including a requested format and
// the output directory.
func RequireBundleKey(producerName, version, format, outputDir string) string {
	return withOutput(withFormat(fmt.Sprintf("%s|%s|bundle", producerName, version), format), outputDir)
}

func withFormat(key, format string) string {
	if format == "" {
		return key
	}
	return key + "|format=" + format
}

func withOutput(key, outputDir string) string {
	return key + "|out=" + filepath.ToSlash(filepath.Clean(outputDir))
}
