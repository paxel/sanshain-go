package config

import (
	"bytes"
	"errors"
	"io"
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

	cfg, err := LoadConfig(path, io.Discard)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.SanshainURL != "http://localhost:8080" {
		t.Errorf("expected http://localhost:8080, got %s", cfg.SanshainURL)
	}
	if cfg.ServiceName != "test-service" {
		t.Errorf("expected test-service, got %s", cfg.ServiceName)
	}
	if want := filepath.Join(filepath.Dir(path), "api.yaml"); cfg.Provide.OpenApiFile != want {
		t.Errorf("expected %s, got %s", want, cfg.Provide.OpenApiFile)
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

	_, err := LoadConfig(path, io.Discard)
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
			_, err := LoadConfig(path, io.Discard)
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
	cfg, err := LoadConfig(path, io.Discard)
	if err != nil {
		t.Fatalf("a retired entry without a file must parse, got: %v", err)
	}
	if len(cfg.Provides) != 1 || !cfg.Provides[0].Retired || cfg.Provides[0].ApiType != "asyncapi" {
		t.Errorf("unexpected config: %+v", cfg.Provides)
	}
}

// Relative paths resolve against the directory holding sanshain.yaml, never the
// working directory; absolute paths stay as they are.
func TestLoadConfig_PathsRelativeToConfigFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(t.TempDir(), "abs.proto")
	path := filepath.Join(dir, "sanshain.yaml")
	content := `
sanshainUrl: http://localhost:8080
serviceName: svc
catalogInfo: catalog-info.yaml
provide:
  file: api/openapi.yaml
provides:
  - openApiFile: ../shared/openapi.yaml
    asyncApiFile: async.yaml
    protoFile: ` + abs + `
  - apiType: asyncapi
    retired: true
requires:
  - serviceName: other
    version: 1.0.0
    outputDirectory: gen/other
    endpoints:
      - method: GET
        path: /x
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path, io.Discard)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	checks := []struct{ got, want string }{
		{cfg.CatalogInfo, filepath.Join(dir, "catalog-info.yaml")},
		{cfg.Provide.File, filepath.Join(dir, "api", "openapi.yaml")},
		{cfg.Provides[0].OpenApiFile, filepath.Join(filepath.Dir(dir), "shared", "openapi.yaml")},
		{cfg.Provides[0].AsyncApiFile, filepath.Join(dir, "async.yaml")},
		{cfg.Provides[0].ProtoFile, abs},
		{cfg.Provides[1].File, ""},
		{cfg.Requires[0].OutputDirectory, filepath.Join(dir, "gen", "other")},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("expected %q, got %q", c.want, c.got)
		}
	}
}

// SANSHAIN_URL beats the yaml sanshainUrl and makes it optional; without it
// the yaml value is used.
func TestLoadConfig_SanshainURLPrecedence(t *testing.T) {
	withURL := writeConfig(t, "sanshainUrl: http://from-yaml\nserviceName: svc\n")
	withoutURL := writeConfig(t, "serviceName: svc\n")

	t.Run("env beats yaml", func(t *testing.T) {
		t.Setenv("SANSHAIN_URL", "http://from-env")
		cfg, err := LoadConfig(withURL, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.SanshainURL != "http://from-env" {
			t.Errorf("expected env URL, got %s", cfg.SanshainURL)
		}
	})

	t.Run("yaml used when env unset", func(t *testing.T) {
		t.Setenv("SANSHAIN_URL", "")
		cfg, err := LoadConfig(withURL, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.SanshainURL != "http://from-yaml" {
			t.Errorf("expected yaml URL, got %s", cfg.SanshainURL)
		}
	})

	t.Run("env alone suffices", func(t *testing.T) {
		t.Setenv("SANSHAIN_URL", "http://from-env")
		cfg, err := LoadConfig(withoutURL, io.Discard)
		if err != nil {
			t.Fatalf("SANSHAIN_URL must satisfy the URL requirement, got %v", err)
		}
		if cfg.SanshainURL != "http://from-env" {
			t.Errorf("expected env URL, got %s", cfg.SanshainURL)
		}
	})

	t.Run("neither fails", func(t *testing.T) {
		t.Setenv("SANSHAIN_URL", "")
		if _, err := LoadConfig(withoutURL, io.Discard); err == nil {
			t.Error("expected an error without any URL")
		}
	})
}

func TestUnknownFields(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want []string
	}{
		{"single top-level", "sanshainUrl: http://x\ncatalogInf: a.yaml\n", []string{"catalogInf"}},
		{"provide object", "sanshainUrl: http://x\nprovide:\n  file: a.yaml\n  fiel: b.yaml\n", []string{"provide.fiel"}},
		{
			name: "several in file order, nested",
			yaml: `
sanshainUrl: http://x
catalogInf: a.yaml
provides:
  - file: a.yaml
  - file: b.yaml
    verison: 1
requires:
  - serviceName: o
    version: 1.0.0
    outputDirectroy: gen
    outputDirectory: gen
    endpoints:
      - method: GET
        path: /a
      - method: GET
        path: /b
      - methd: GET
        path: /c
zzz: 1
`,
			want: []string{"catalogInf", "provides[1].verison", "requires[0].outputDirectroy", "requires[0].endpoints[2].methd", "zzz"},
		},
		{
			name: "known but unsupported fields are accepted silently",
			yaml: `
sanshainUrl: http://x
force: true
insecure: true
combine: true
catalogInfo: catalog-info.yaml
provides:
  - file: a.yaml
    combine: true
`,
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := unknownFields([]byte(tc.yaml))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

// The cat appears as soon as the YAML parses: after the legacy rejection,
// before any other validation error the typo may cause.
func TestLoadConfig_CatBeforeOtherValidation(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		field   string
		wantErr string
	}{
		{"sanshainUrl typo", "sanshainUrll: http://x\nserviceName: svc\n", "'sanshainUrll'", "missing required field: sanshainUrl"},
		{"provide file typo", "sanshainUrl: http://x\nprovide: {fiel: a.yaml}\n", "'provide.fiel'", "at least one of file, openApiFile, asyncApiFile, or protoFile"},
		{"requires version typo", `
sanshainUrl: http://x
requires:
  - serviceName: o
    verison: 1.0.0
    outputDirectory: gen
    endpoints:
      - method: GET
        path: /a
`, "'requires[0].verison'", "missing 'version'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SANSHAIN_URL", "")
			var warn bytes.Buffer
			_, err := LoadConfig(writeConfig(t, tc.yaml), &warn)
			want := "sanshain.yaml: unknown field " + tc.field + " - ignored"
			if !strings.Contains(warn.String(), want) {
				t.Errorf("expected cat with %q, got %q", want, warn.String())
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected the validation error %q after the cat, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestLoadConfig_StrictUnknownFailsBeforeOtherValidation(t *testing.T) {
	t.Setenv("SANSHAIN_URL", "")
	var warn bytes.Buffer
	_, err := LoadConfig(writeConfig(t, "strict: true\nsanshainUrll: http://x\n"), &warn)
	if !errors.Is(err, ErrUnknownFieldsStrict) {
		t.Errorf("expected the strict unknown-field error, got %v", err)
	}
	if !strings.Contains(warn.String(), "unknown field 'sanshainUrll' - rejected (strict)") {
		t.Errorf("unexpected cat %q", warn.String())
	}
}

func TestLoadConfig_NoCatWithoutUnknownFields(t *testing.T) {
	var warn bytes.Buffer
	if _, err := LoadConfig(writeConfig(t, "sanshainUrl: http://x\nforce: true\ninsecure: true\n"), &warn); err != nil {
		t.Fatal(err)
	}
	if warn.Len() != 0 {
		t.Errorf("expected no cat, got %q", warn.String())
	}
}

func TestLoadConfig_LegacyFieldErrorsBeforeUnknownFields(t *testing.T) {
	var warn bytes.Buffer
	path := writeConfig(t, "sanshainUrl: http://x\ncatalogInf: a.yaml\nbaseVersion: 3\n")
	if _, err := LoadConfig(path, &warn); err == nil || !strings.Contains(err.Error(), "'baseVersion' is no longer supported") {
		t.Errorf("expected the legacy rejection, got %v", err)
	}
	if warn.Len() != 0 {
		t.Errorf("no cat may precede the legacy error, got %q", warn.String())
	}
}

func TestPuzzledCat(t *testing.T) {
	single := "   /\\_/\\  ?\n  ( O_o )   sanshain.yaml: unknown field 'catalogInf' - ignored\n ---U-U---\n"
	if got := PuzzledCat([]string{"catalogInf"}, false); got != single {
		t.Errorf("expected\n%s\ngot\n%s", single, got)
	}
	several := "   /\\_/\\  ?\n  ( O_o )   sanshain.yaml: unknown fields 'catalogInf', 'requires[0].verison' - ignored\n ---U-U---\n"
	if got := PuzzledCat([]string{"catalogInf", "requires[0].verison"}, false); got != several {
		t.Errorf("expected\n%s\ngot\n%s", several, got)
	}
	strict := "   /\\_/\\  ?\n  ( O_o )   sanshain.yaml: unknown field 'catalogInf' - rejected (strict)\n ---U-U---\n"
	if got := PuzzledCat([]string{"catalogInf"}, true); got != strict {
		t.Errorf("expected\n%s\ngot\n%s", strict, got)
	}
}
