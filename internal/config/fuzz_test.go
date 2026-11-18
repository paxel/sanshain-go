package config

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"
)

func TestParseConfig_ReadsTheContentAndResolvesAgainstTheBaseDir(t *testing.T) {
	t.Setenv("SANSHAIN_URL", "")
	base := t.TempDir()
	cfg, err := parseConfig([]byte("sanshainUrl: http://localhost:8080\nserviceName: svc\nprovides:\n  - file: api/openapi.yaml\n"), base, io.Discard, false)
	if err != nil {
		t.Fatalf("expected the config to load, got %v", err)
	}
	if cfg.ServiceName != "svc" || cfg.Provides[0].File != filepath.Join(base, "api/openapi.yaml") {
		t.Errorf("unexpected config %+v", cfg)
	}
}

func TestParseConfig_ReportsUnknownFields(t *testing.T) {
	t.Setenv("SANSHAIN_URL", "")
	var warn bytes.Buffer
	if _, err := parseConfig([]byte("sanshainUrl: http://localhost:8080\nserviceNam: svc\n"), ".", &warn, false); err != nil {
		t.Fatalf("expected the config to load, got %v", err)
	}
	if !bytes.Contains(warn.Bytes(), []byte("'serviceNam'")) {
		t.Errorf("expected the unknown field to be named, got %q", warn.String())
	}
}

// FuzzConfig reads sanshain.yaml as any content: a configuration or an error,
// never a panic, and never both.
func FuzzConfig(f *testing.F) {
	f.Add([]byte("sanshainUrl: http://localhost:8080\nserviceName: svc\nprovides:\n  - file: api/openapi.yaml\n"), false)
	f.Add([]byte("sanshainUrl: http://localhost:8080\nstrict: true\ntypo: 1\nrequires:\n  - serviceName: auth\n    version: 1.2.0\n    outputDirectory: gen\n    endpoints:\n      - method: GET\n        path: /a\n"), true)
	f.Add([]byte("sanshainUrl: http://localhost:8080\nannotations:\n  - scope: version\n    name: build\n    value: \"{{ID}}\"\n  - scope: 42\n"), false)
	f.Add([]byte("branch: main\nprovide:\n  timeout: 3\n"), false)
	f.Add([]byte("- a\n- [b\n"), true)
	f.Add([]byte(""), false)
	f.Fuzz(func(t *testing.T, data []byte, bestEffort bool) {
		cfg, err := parseConfig(data, ".", io.Discard, bestEffort)
		if (cfg == nil) == (err == nil) {
			t.Fatalf("expected a configuration or an error, got %v and %v", cfg, err)
		}
	})
}
