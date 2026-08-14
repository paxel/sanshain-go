package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sanshain.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	path := writeConfig(t, `
sanshainUrl: http://localhost:8080
serviceName: test-service
provide:
  openApiFile: api.yaml
requires:
  - serviceName: other-service
    version: 1.2.0
    outputDirectory: api
    endpoints:
      - method: GET
        path: /health
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.SanshainURL != "http://localhost:8080" {
		t.Errorf("expected http://localhost:8080, got %s", cfg.SanshainURL)
	}
	if cfg.ServiceName != "test-service" {
		t.Errorf("expected test-service, got %s", cfg.ServiceName)
	}
	if cfg.Provide.OpenApiFile != "api.yaml" {
		t.Errorf("expected api.yaml, got %s", cfg.Provide.OpenApiFile)
	}
	if len(cfg.Requires) != 1 {
		t.Fatalf("expected 1 require, got %d", len(cfg.Requires))
	}
	if cfg.Requires[0].Version != "1.2.0" {
		t.Errorf("expected pinned version 1.2.0, got %s", cfg.Requires[0].Version)
	}
}

func TestLoadConfig_MissingRequireVersion(t *testing.T) {
	path := writeConfig(t, `
sanshainUrl: http://localhost:8080
serviceName: test-service
requires:
  - serviceName: user-service
    outputDirectory: api
    endpoints:
      - method: GET
        path: /health
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing version")
	}
	msg := err.Error()
	expected := "requires[0] (user-service): missing 'version' — Sanshain 2.0 pins exact versions; add version: 1.2.0 (list available: GET /producers/user-service/versions)"
	if msg != expected {
		t.Errorf("expected %q, got %q", expected, msg)
	}
}

func TestLoadConfig_BranchEraFieldsRejectedByName(t *testing.T) {
	cases := []struct {
		name     string
		yaml     string
		contains []string
	}{
		{
			name: "branch in requires",
			yaml: `
sanshainUrl: http://localhost:8080
serviceName: s
requires:
  - serviceName: user-service
    branch: main
    version: 1.0.0
    outputDirectory: api
    endpoints:
      - method: GET
        path: /health
`,
			contains: []string{
				"requires[0]",
				"'branch' is no longer supported",
				"branch model was removed in Sanshain 2.0",
				"'version' pin",
			},
		},
		{
			name: "branch in provide",
			yaml: `
sanshainUrl: http://localhost:8080
serviceName: s
provide:
  file: api.yaml
  branch: main
`,
			contains: []string{"provide", "'branch' is no longer supported"},
		},
		{
			name: "baseVersion in provides",
			yaml: `
sanshainUrl: http://localhost:8080
serviceName: s
provides:
  - file: api.yaml
    baseVersion: 5
`,
			contains: []string{
				"provides[0]",
				"'baseVersion' is no longer supported",
				"optimistic concurrency was removed in Sanshain 2.0",
			},
		},
		{
			name: "top-level timeout",
			yaml: `
sanshainUrl: http://localhost:8080
serviceName: s
timeout: 30
`,
			contains: []string{
				"'timeout' is no longer supported",
				"never waits",
			},
		},
		{
			name: "timeout in requires",
			yaml: `
sanshainUrl: http://localhost:8080
serviceName: s
requires:
  - serviceName: user-service
    version: 1.0.0
    timeout: 30
    outputDirectory: api
    endpoints:
      - method: GET
        path: /health
`,
			contains: []string{"requires[0]", "'timeout' is no longer supported"},
		},
		{
			name: "top-level releaseBranches",
			yaml: `
sanshainUrl: http://localhost:8080
serviceName: s
releaseBranches:
  - main
`,
			contains: []string{
				"'releaseBranches' is no longer supported",
				"SANSHAIN_GA=true",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfig(t, tc.yaml)
			_, err := LoadConfig(path)
			if err == nil {
				t.Fatal("expected branch-era field to be rejected")
			}
			for _, want := range tc.contains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected error to contain %q, got %q", want, err.Error())
				}
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	t.Run("Missing sanshainUrl", func(t *testing.T) {
		cfg := &SanshainConfig{ServiceName: "test"}
		if err := validateConfig(cfg); err == nil {
			t.Error("expected error for missing sanshainUrl")
		}
	})

	t.Run("Missing serviceName is allowed", func(t *testing.T) {
		cfg := &SanshainConfig{SanshainURL: "http://test"}
		if err := validateConfig(cfg); err != nil {
			t.Errorf("serviceName should be optional, got error: %v", err)
		}
	})
}

// A retired entry names its family via apiType alone — requiring a file would
// force projects to keep a dead spec on disk forever.
func TestLoadConfig_RetiredEntryNeedsNoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sanshain.yaml")
	yaml := `
sanshainUrl: http://localhost:8080
serviceName: svc
provides:
  - apiType: asyncapi
    retired: true
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("a retired entry without a file must parse, got: %v", err)
	}
	if len(cfg.Provides) != 1 || !cfg.Provides[0].Retired || cfg.Provides[0].ApiType != "asyncapi" {
		t.Errorf("unexpected config: %+v", cfg.Provides)
	}
}
