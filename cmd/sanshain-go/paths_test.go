package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/paxel/sanshain-go/v2/internal/api"
	"github.com/paxel/sanshain-go/v2/internal/cache"
	"github.com/paxel/sanshain-go/v2/internal/config"
)

// writeFile creates parent directories and writes content to path.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// A config in another folder (as with --config) finds its spec file and writes
// its required spec next to itself, not into the working directory.
func TestConfigInOtherFolder_ResolvesSpecAndOutputNextToConfig(t *testing.T) {
	var provided string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/provide":
			var body map[string]any
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &body)
			provided, _ = body["openapi_yaml"].(string)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"version":"1.0.0","stability":"snapshot","changes":{}}`))
		case "/require":
			_, _ = w.Write([]byte("openapi: required"))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer ts.Close()

	dir := filepath.Join(t.TempDir(), "project", "build")
	configPath := filepath.Join(dir, "sanshain.yaml")
	writeFile(t, filepath.Join(dir, "api", "openapi.yaml"), "openapi: next-to-config")
	writeFile(t, configPath, `
sanshainUrl: `+ts.URL+`
serviceName: svc
provide:
  file: api/openapi.yaml
requires:
  - serviceName: other
    version: 1.0.0
    outputDirectory: gen
    endpoints:
      - method: GET
        path: /x
`)

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	client := api.NewSanshainClient(ts.URL, "", false)

	if err := handleProvide(client, cfg, "snapshot", api.Stream{}); err != nil {
		t.Fatalf("provide failed: %v", err)
	}
	if provided != "openapi: next-to-config" {
		t.Errorf("expected the spec next to the config to be sent, got %q", provided)
	}

	if err := handleRequire(client, cfg, cache.NewSanshainCache(t.TempDir()), api.Stream{}); err != nil {
		t.Fatalf("require failed: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "gen", "other.yaml"))
	if err != nil {
		t.Fatalf("expected required spec next to the config: %v", err)
	}
	if string(got) != "openapi: required" {
		t.Errorf("unexpected required content %q", got)
	}
}
