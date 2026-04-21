package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	content := `
sanshainUrl: http://localhost:8080
clientName: test-client
provide:
  serviceName: test-service
  openApiFile: api.yaml
requires:
  - serviceName: other-service
    outputDirectory: api
    endpoints:
      - method: GET
        path: /health
`
	tmpfile, err := os.CreateTemp("", "sanshain.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.SanshainURL != "http://localhost:8080" {
		t.Errorf("expected http://localhost:8080, got %s", cfg.SanshainURL)
	}
	if cfg.ClientName != "test-client" {
		t.Errorf("expected test-client, got %s", cfg.ClientName)
	}
	if cfg.Provide.ServiceName != "test-service" {
		t.Errorf("expected test-service, got %s", cfg.Provide.ServiceName)
	}
	if len(cfg.Requires) != 1 {
		t.Errorf("expected 1 require, got %d", len(cfg.Requires))
	}
}

func TestValidateConfig(t *testing.T) {
	t.Run("Missing sanshainUrl", func(t *testing.T) {
		cfg := &SanshainConfig{ClientName: "test"}
		if err := validateConfig(cfg); err == nil {
			t.Error("expected error for missing sanshainUrl")
		}
	})

	t.Run("Missing clientName", func(t *testing.T) {
		cfg := &SanshainConfig{SanshainURL: "http://test"}
		if err := validateConfig(cfg); err == nil {
			t.Error("expected error for missing clientName")
		}
	})
}
