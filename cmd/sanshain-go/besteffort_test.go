package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// failingServer refuses the OpenAPI provide and any require of the Producer
// ghost with a 500, and answers everything else.
func failingServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/version":
			_, _ = w.Write([]byte(`{"version":"2.13.0"}`))
		case r.URL.Path == "/provide" || r.URL.Query().Get("producername") == "ghost":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		case strings.HasPrefix(r.URL.Path, "/provide"):
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"version":"1.0.0","stability":"snapshot","changes":{}}`))
		default:
			_, _ = w.Write([]byte(openapiDoc))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A failed provide under best effort warns and the next entry is provided.
func TestBestEffort_ProvideGoesOnWithTheNextEntry(t *testing.T) {
	srv := failingServer(t)
	dir := filepath.Join(t.TempDir(), "conf")
	writeFile(t, filepath.Join(dir, "openapi.yaml"), openapiDoc)
	writeFile(t, filepath.Join(dir, "asyncapi.yaml"), asyncapiDoc)
	cfgPath := filepath.Join(dir, "sanshain.yaml")
	writeFile(t, cfgPath, "sanshainUrl: "+srv.URL+"\nserviceName: svc\nprovides:\n  - file: openapi.yaml\n  - file: asyncapi.yaml\n    apiType: asyncapi\n")

	stdout, _, code := runMain(t, nil, "--config", cfgPath, "--best-effort", "provide")
	if code != 0 {
		t.Fatalf("expected exit 0 under best effort, got %d: %s", code, stdout)
	}
	if !strings.Contains(stdout, "Warning: ") || !strings.Contains(stdout, "boom") {
		t.Errorf("expected the failed provide as a warning, got %q", stdout)
	}
	if !strings.Contains(stdout, "✓ Provided svc v1.0.0") {
		t.Errorf("expected the second entry provided, got %q", stdout)
	}

	_, _, code = runMain(t, nil, "--config", cfgPath, "provide")
	if code == 0 {
		t.Error("without best effort a failed provide fails the build")
	}
}

// A failed require under best effort warns and the next entry is written.
func TestBestEffort_RequireGoesOnWithTheNextEntry(t *testing.T) {
	srv := failingServer(t)
	cfgPath, dir := requireProject(t, srv.URL, `  - serviceName: ghost
    version: 1.0.0
    outputDirectory: gen
    endpoints:
      - method: GET
        path: /x
  - serviceName: users
    version: 1.0.0
    outputDirectory: gen
    endpoints:
      - method: GET
        path: /y
`)
	stdout, _, code := runMain(t, nil, "--config", cfgPath, "--best-effort", "require")
	if code != 0 {
		t.Fatalf("expected exit 0 under best effort, got %d: %s", code, stdout)
	}
	if !strings.Contains(stdout, "Warning: failed to require ghost") {
		t.Errorf("expected the failed require as a warning, got %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "gen", "users.yaml")); err != nil {
		t.Errorf("expected the second entry written: %v", err)
	}
}

// A missing or broken sanshain.yaml fails without best effort, naming the
// error; with --best-effort or SANSHAIN_BEST_EFFORT it is a warning, exit 0
// and no call. The file's own bestEffort does not count for it.
func TestBestEffort_AMissingOrBrokenConfig(t *testing.T) {
	broken := map[string]string{
		"missing":                  "",
		"invalid YAML":             "sanshainUrl: [http://127.0.0.1:1\nserviceName: svc\n",
		"failed validation":        "serviceName: svc\nrequires:\n  - serviceName: users\n    outputDirectory: gen\n    endpoints:\n      - method: GET\n        path: /x\n",
		"1.x field":                "sanshainUrl: http://127.0.0.1:1\nserviceName: svc\nreleaseBranches: [main]\n",
		"unsupported apiType":      "sanshainUrl: http://127.0.0.1:1\nserviceName: svc\nprovides:\n  - file: openapi.yaml\n    apiType: raml\n",
		"unknown requires apiType": "sanshainUrl: http://127.0.0.1:1\nserviceName: svc\nrequires:\n  - serviceName: users\n    version: 1.0.0\n    apiType: raml\n    outputDirectory: gen\n    endpoints:\n      - method: GET\n        path: /x\n",
	}
	modes := []struct {
		name string
		env  []string
		args []string
		ok   bool
	}{
		{"off", nil, nil, false},
		{"flag", nil, []string{"--best-effort"}, true},
		{"env", []string{"SANSHAIN_BEST_EFFORT=true"}, nil, true},
		{"env false", []string{"SANSHAIN_BEST_EFFORT=false"}, nil, false},
	}
	for name, yaml := range broken {
		for _, mode := range modes {
			for _, command := range []string{"provide", "require", "catalog"} {
				t.Run(name+"/"+mode.name+"/"+command, func(t *testing.T) {
					cfgPath := filepath.Join(t.TempDir(), "sanshain.yaml")
					if yaml != "" {
						writeFile(t, cfgPath, yaml)
					}
					args := append(append([]string{"--config", cfgPath}, mode.args...), command)
					stdout, _, code := runMain(t, mode.env, args...)
					if !mode.ok {
						if code != 1 || !strings.HasPrefix(stdout, "Error loading config: ") {
							t.Fatalf("expected the config error and exit 1, got %d %q", code, stdout)
						}
						if yaml == "" && !strings.Contains(stdout, cfgPath) {
							t.Errorf("expected the missing file named, got %q", stdout)
						}
						return
					}
					if code != 0 {
						t.Fatalf("expected exit 0 under best effort, got %d %q", code, stdout)
					}
					// One warning line and nothing else: no call, no skip notice.
					if !strings.HasPrefix(stdout, "Warning: ") || strings.Count(stdout, "\n") != 1 {
						t.Errorf("expected only the config error as a warning, got %q", stdout)
					}
					if yaml == "" && !strings.Contains(stdout, cfgPath) {
						t.Errorf("expected the missing file named, got %q", stdout)
					}
				})
			}
		}
	}
}

// The broken file's own bestEffort does not soften it.
func TestBestEffort_ABrokenConfigsOwnBestEffortDoesNotCount(t *testing.T) {
	for name, yaml := range map[string]string{
		"invalid YAML":        "bestEffort: true\nsanshainUrl: [http://127.0.0.1:1\n",
		"1.x field":           "sanshainUrl: http://127.0.0.1:1\nserviceName: svc\nbestEffort: true\nbranch: main\n",
		"unsupported apiType": "sanshainUrl: http://127.0.0.1:1\nserviceName: svc\nbestEffort: true\nprovides:\n  - file: openapi.yaml\n    apiType: raml\n",
	} {
		t.Run(name, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "sanshain.yaml")
			writeFile(t, cfgPath, yaml)
			stdout, _, code := runMain(t, nil, "--config", cfgPath, "provide")
			if code != 1 || !strings.HasPrefix(stdout, "Error loading config: ") {
				t.Fatalf("expected the config error and exit 1, got %d %q", code, stdout)
			}
		})
	}
}

// Best effort for a broken file is a noop: no request reaches the service.
func TestBestEffort_ABrokenConfigMakesNoCall(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "openapi.yaml"), openapiDoc)
	cfgPath := filepath.Join(dir, "sanshain.yaml")
	writeFile(t, cfgPath, "sanshainUrl: "+srv.URL+"\nserviceName: svc\nbranch: main\nprovides:\n  - file: openapi.yaml\n")
	stdout, _, code := runMain(t, nil, "--config", cfgPath, "--best-effort", "provide")
	if code != 0 || calls.Load() != 0 {
		t.Errorf("expected exit 0 and no call, got %d with %d calls: %q", code, calls.Load(), stdout)
	}
}

// Missing configuration under strict fails also with best effort, in every
// command.
func TestBestEffort_StrictMissingConfigFails(t *testing.T) {
	cases := []struct {
		command, yaml, want string
	}{
		{"provide", "strict: true\n", "serviceName is required"},
		{"require", "strict: true\n", "serviceName is required"},
		{"catalog", "strict: true\n", "serviceName is required"},
		{"provide", "strict: true\nserviceName: svc\n", "no provide configuration found"},
		{"require", "strict: true\nserviceName: svc\n", "no requires configured"},
	}
	for _, tc := range cases {
		t.Run(tc.command+" "+tc.want, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "sanshain.yaml")
			writeFile(t, cfgPath, "sanshainUrl: http://127.0.0.1:1\n"+tc.yaml)
			stdout, _, code := runMain(t, nil, "--config", cfgPath, "--best-effort", tc.command)
			if code == 0 || !strings.Contains(stdout, "Error: "+tc.want) {
				t.Errorf("expected strict to fail with %q under best effort, got %d %q", tc.want, code, stdout)
			}
		})
	}
}
