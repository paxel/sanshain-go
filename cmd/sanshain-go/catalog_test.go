package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// catalogServer records the catalog pushes it receives and answers each with
// status/body; GET /version answers version (404 when empty).
type catalogServer struct {
	*httptest.Server
	mu      sync.Mutex
	bodies  []map[string]any
	paths   []string
	methods []string
}

func newCatalogServer(t *testing.T, status int, body, version string) *catalogServer {
	t.Helper()
	cs := &catalogServer{}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			if version == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`{"version":"` + version + `"}`))
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var decoded map[string]any
		_ = json.Unmarshal(raw, &decoded)
		cs.mu.Lock()
		cs.bodies = append(cs.bodies, decoded)
		cs.paths = append(cs.paths, r.URL.EscapedPath())
		cs.methods = append(cs.methods, r.Method)
		cs.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(cs.Close)
	return cs
}

const recorded = `{"system":"orders-service","trunk":true,"resources":["resource:default/orders-db","resource:default/orders-bucket"]}`

// catalogProject writes catalog-info.yaml and a sanshain.yaml next to it in a
// folder other than the working directory, and returns the config path.
func catalogProject(t *testing.T, url, extra string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "project", "conf")
	writeFile(t, filepath.Join(dir, "backstage", "catalog-info.yaml"), "kind: Component\nmetadata:\n  name: orders-service\n")
	cfgPath := filepath.Join(dir, "sanshain.yaml")
	writeFile(t, cfgPath, "sanshainUrl: "+url+"\nserviceName: orders service\n"+extra)
	return cfgPath
}

const withCatalog = "catalogInfo: backstage/catalog-info.yaml\n"

func TestCatalog_StreamFlags(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantTrunk bool
	}{
		{"trunk build sends trunk", []string{"--trunk"}, true},
		{"plain build omits trunk", nil, false},
		{"tag build sends neither trunk nor tag", []string{"--tag", "R1"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := newCatalogServer(t, http.StatusOK, recorded, "2.4.0")
			cfgPath := catalogProject(t, cs.URL, withCatalog)
			args := append(append([]string{"--config", cfgPath}, tc.args...), "catalog")
			stdout, _, code := runMain(t, nil, args...)
			if code != 0 {
				t.Fatalf("expected exit 0, got %d: %s", code, stdout)
			}
			if len(cs.bodies) != 1 {
				t.Fatalf("expected one push, got %d", len(cs.bodies))
			}
			body := cs.bodies[0]
			if cs.methods[0] != http.MethodPut || cs.paths[0] != "/catalog/orders%20service" {
				t.Errorf("expected PUT /catalog/orders%%20service, got %s %s", cs.methods[0], cs.paths[0])
			}
			if body["catalog_info_yaml"] != "kind: Component\nmetadata:\n  name: orders-service\n" {
				t.Errorf("expected the file next to the config, got %v", body["catalog_info_yaml"])
			}
			trunk, present := body["trunk"]
			if tc.wantTrunk && trunk != true {
				t.Errorf("expected trunk: true, got %v", body)
			}
			if !tc.wantTrunk && present {
				t.Errorf("trunk must be omitted, got %v", body)
			}
			if _, present := body["tag"]; present {
				t.Errorf("tag must never be sent, got %v", body)
			}
		})
	}
}

func TestCatalog_RecordedOutput(t *testing.T) {
	cs := newCatalogServer(t, http.StatusOK, recorded, "2.4.0")
	stdout, _, code := runMain(t, nil, "--config", catalogProject(t, cs.URL, withCatalog), "--trunk", "catalog")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout)
	}
	want := "Catalog recorded for orders-service (main+dev): 2 resources\n  resource:default/orders-db\n  resource:default/orders-bucket\n"
	if !strings.Contains(stdout, want) {
		t.Errorf("expected %q in output, got %q", want, stdout)
	}

	dev := newCatalogServer(t, http.StatusOK, `{"system":"orders-service","trunk":false,"resources":[]}`, "2.4.0")
	stdout, _, _ = runMain(t, nil, "--config", catalogProject(t, dev.URL, withCatalog), "catalog")
	if !strings.Contains(stdout, "Catalog recorded for orders-service (dev): 0 resources\n") {
		t.Errorf("unexpected dev output %q", stdout)
	}
}

// A 400 can only come from a service that predates the Sanshain-Warning
// header: it gets the generic error mapping.
func TestCatalog_400UsesNormalMapping(t *testing.T) {
	cs := newCatalogServer(t, http.StatusBadRequest, `{"error":"no Component named 'orders service'"}`, "2.4.0")
	stdout, _, code := runMain(t, nil, "--config", catalogProject(t, cs.URL, withCatalog), "catalog")
	if code == 0 || !strings.Contains(stdout, "Error: request failed with status 400") {
		t.Errorf("expected the normal error mapping, got %d %q", code, stdout)
	}
}

func TestCatalog_BestEffortTurns400IntoWarning(t *testing.T) {
	cs := newCatalogServer(t, http.StatusBadRequest, `{"error":"bad"}`, "2.4.0")
	stdout, _, code := runMain(t, nil, "--config", catalogProject(t, cs.URL, withCatalog), "--best-effort", "catalog")
	if code != 0 {
		t.Fatalf("expected exit 0 under best-effort, got %d", code)
	}
	if !strings.Contains(stdout, "Warning: request failed with status 400") {
		t.Errorf("expected a warning, got %q", stdout)
	}
}

func TestCatalog_404NeedsService240(t *testing.T) {
	cs := newCatalogServer(t, http.StatusNotFound, `{"error":"not found"}`, "2.3.0")
	stdout, _, code := runMain(t, nil, "--config", catalogProject(t, cs.URL, withCatalog), "catalog")
	if code == 0 {
		t.Fatal("expected a 404 to fail")
	}
	if !strings.Contains(stdout, "sanshain catalog needs Sanshain Service 2.4.0 or later (server is 2.3.0)") {
		t.Errorf("unexpected output %q", stdout)
	}

	unknown := newCatalogServer(t, http.StatusNotFound, `{"error":"not found"}`, "")
	stdout, _, _ = runMain(t, nil, "--config", catalogProject(t, unknown.URL, withCatalog), "catalog")
	if !strings.Contains(stdout, "sanshain catalog needs Sanshain Service 2.4.0 or later (server version unknown)") {
		t.Errorf("unexpected output %q", stdout)
	}
}

func TestCatalog_OtherFailureUsesNormalMapping(t *testing.T) {
	cs := newCatalogServer(t, http.StatusUnauthorized, `{"error":"no token"}`, "2.4.0")
	stdout, _, code := runMain(t, nil, "--config", catalogProject(t, cs.URL, withCatalog), "catalog")
	if code == 0 || !strings.Contains(stdout, "request failed with status 401") {
		t.Errorf("expected the normal error mapping, got %d %q", code, stdout)
	}
}

func TestCatalog_NoCatalogInfo(t *testing.T) {
	cs := newCatalogServer(t, http.StatusOK, recorded, "2.4.0")

	stdout, _, code := runMain(t, nil, "--config", catalogProject(t, cs.URL, ""), "catalog")
	if code != 0 || !strings.Contains(stdout, "No catalogInfo in sanshain.yaml - skipping catalog.") {
		t.Errorf("expected a warning and exit 0, got %d %q", code, stdout)
	}

	stdout, _, code = runMain(t, nil, "--config", catalogProject(t, cs.URL, "strict: true\n"), "--best-effort", "catalog")
	if code == 0 || !strings.Contains(stdout, "strict: sanshain.yaml has no catalogInfo") {
		t.Errorf("expected strict to fail even under best-effort, got %d %q", code, stdout)
	}
	if len(cs.bodies) != 0 {
		t.Errorf("nothing may be pushed without catalogInfo, got %v", cs.bodies)
	}
}

// Strict for catalog checks only serviceName and catalogInfo: no provides or
// requires is fine.
func TestCatalog_StrictIgnoresMissingProvidesAndRequires(t *testing.T) {
	cs := newCatalogServer(t, http.StatusOK, recorded, "2.4.0")
	stdout, _, code := runMain(t, nil, "--config", catalogProject(t, cs.URL, "strict: true\n"+withCatalog), "catalog")
	if code != 0 {
		t.Errorf("expected exit 0, got %d %q", code, stdout)
	}
}

func TestCatalog_MissingFileNamesResolvedPath(t *testing.T) {
	cs := newCatalogServer(t, http.StatusOK, recorded, "2.4.0")
	cfgPath := catalogProject(t, cs.URL, "catalogInfo: missing.yaml\n")
	resolved := filepath.Join(filepath.Dir(cfgPath), "missing.yaml")

	stdout, _, code := runMain(t, nil, "--config", cfgPath, "catalog")
	if code == 0 || !strings.Contains(stdout, resolved) {
		t.Errorf("expected a failure naming %s, got %d %q", resolved, code, stdout)
	}

	stdout, _, code = runMain(t, []string{"SANSHAIN_BEST_EFFORT=1"}, "--config", cfgPath, "catalog")
	if code != 0 || !strings.Contains(stdout, "Warning: failed to read catalogInfo file "+resolved) {
		t.Errorf("expected a best-effort warning, got %d %q", code, stdout)
	}
}
