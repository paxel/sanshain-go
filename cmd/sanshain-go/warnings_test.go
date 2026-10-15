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

const noBranch = "no sanshain-branch 'R9' - this build was not recorded in a release graph"
const neither = "trunk and tag are mutually exclusive - this build is recorded in neither"

// seenRequest is one call a warnServer received.
type seenRequest struct {
	path  string
	query string
	body  map[string]any
}

// warnServer answers every call with status, a body fitting the endpoint and
// one Sanshain-Warning header line per warning; GET /version answers 2.4.0.
type warnServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []seenRequest
}

func newWarnServer(t *testing.T, status int, catalogBody string, warnings ...string) *warnServer {
	t.Helper()
	ws := &warnServer{}
	ws.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = w.Write([]byte(`{"version":"2.4.0"}`))
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var decoded map[string]any
		_ = json.Unmarshal(raw, &decoded)
		ws.mu.Lock()
		ws.requests = append(ws.requests, seenRequest{r.URL.Path, r.URL.RawQuery, decoded})
		ws.mu.Unlock()
		for _, warning := range warnings {
			w.Header().Add("Sanshain-Warning", warning)
		}
		w.WriteHeader(status)
		switch {
		case status >= 300:
			_, _ = w.Write([]byte(`{"error":"no such pin"}`))
		case strings.HasPrefix(r.URL.Path, "/catalog/"):
			_, _ = w.Write([]byte(catalogBody))
		case strings.HasPrefix(r.URL.Path, "/provide") && decoded["retired"] == true:
			_, _ = w.Write([]byte(`{"tag_cleared":"","trunk_pins_closed":0,"contracts_released":0}`))
		case strings.HasPrefix(r.URL.Path, "/provide"):
			_, _ = w.Write([]byte(`{"version":"1.0.0","stability":"snapshot","changes":{}}`))
		default:
			_, _ = w.Write([]byte(openapiDoc))
		}
	}))
	t.Cleanup(ws.Close)
	return ws
}

func (ws *warnServer) seen() []seenRequest {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return append([]seenRequest(nil), ws.requests...)
}

// warnProject writes a sanshain.yaml with a provide, a single-endpoint and a
// bundle require and a catalogInfo, plus extra, and returns its path.
func warnProject(t *testing.T, url, extra string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "conf")
	writeFile(t, filepath.Join(dir, "openapi.yaml"), openapiDoc)
	writeFile(t, filepath.Join(dir, "catalog-info.yaml"), "kind: Component\n")
	cfgPath := filepath.Join(dir, "sanshain.yaml")
	writeFile(t, cfgPath, `sanshainUrl: `+url+`
serviceName: svc
catalogInfo: catalog-info.yaml
provide:
  file: openapi.yaml
requires:
  - serviceName: single
    version: 1.0.0
    outputDirectory: gen
    endpoints:
      - method: GET
        path: /x
  - serviceName: bundle
    version: 1.0.0
    outputDirectory: gen
    endpoints:
      - method: GET
        path: /x
      - method: GET
        path: /y
`+extra)
	return cfgPath
}

const recordedCatalog = `{"system":"svc","trunk":false,"resources":[],"recorded":true}`

// Every call prints each Sanshain-Warning header line once, as received.
func TestWarnings_PrintedForEveryCall(t *testing.T) {
	cases := []struct {
		name    string
		extra   string
		command string
		calls   int
	}{
		{"provide", "", "provide", 1},
		{"retire", "provides:\n  - apiType: asyncapi\n    retired: true\n", "provide", 2},
		{"require and bundle", "", "require", 2},
		{"catalog", "", "catalog", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := newWarnServer(t, http.StatusOK, recordedCatalog, noBranch)
			stdout, _, code := runMain(t, nil, "--config", warnProject(t, ws.URL, tc.extra), "--tag", "R9", tc.command)
			if code != 0 {
				t.Fatalf("a warning never fails a build, got %d: %s", code, stdout)
			}
			if got := strings.Count(stdout, "⚠ "+noBranch+"\n"); got != tc.calls {
				t.Errorf("expected the warning once per call (%d), got %d in %q", tc.calls, got, stdout)
			}
		})
	}
}

// Several header lines on one call are each printed, never split on commas.
func TestWarnings_SeveralOnOneCall(t *testing.T) {
	ws := newWarnServer(t, http.StatusOK, recordedCatalog, "first, with a comma", "second")
	stdout, _, code := runMain(t, nil, "--config", warnProject(t, ws.URL, ""), "catalog")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout)
	}
	if !strings.Contains(stdout, "⚠ first, with a comma\n⚠ second\n") {
		t.Errorf("expected both warnings unsplit, got %q", stdout)
	}
}

// Strict turns the first warning into a failure; best-effort does not soften it.
func TestWarnings_StrictFails(t *testing.T) {
	for _, extra := range [][]string{nil, {"--best-effort"}} {
		ws := newWarnServer(t, http.StatusOK, recordedCatalog, noBranch, "second")
		args := append(append([]string{"--config", warnProject(t, ws.URL, "strict: true\n"), "--tag", "R9"}, extra...), "provide")
		stdout, _, code := runMain(t, nil, args...)
		if code == 0 {
			t.Errorf("%v: strict must fail on a warning, got %q", extra, stdout)
		}
		if !strings.Contains(stdout, "⚠ "+noBranch+"\n⚠ second\nError: strict: Sanshain warned: "+noBranch+"\n") {
			t.Errorf("%v: expected both warnings, then the strict failure naming the first, got %q", extra, stdout)
		}
		if strings.Contains(stdout, "Successfully provided") {
			t.Errorf("%v: strict fails right after the call, got %q", extra, stdout)
		}
	}
}

// A failed call carrying a warning prints the warning; under strict the
// error comes first, then the strict failure.
func TestWarnings_OnErrorResponse(t *testing.T) {
	ws := newWarnServer(t, http.StatusNotFound, "", noBranch)
	stdout, _, code := runMain(t, nil, "--config", warnProject(t, ws.URL, ""), "--tag", "R9", "require")
	if code == 0 || !strings.Contains(stdout, "⚠ "+noBranch+"\nError: failed to require single: unknown producer or version (404)") {
		t.Errorf("expected the warning, then the error, got %d %q", code, stdout)
	}

	ws = newWarnServer(t, http.StatusNotFound, "", noBranch)
	stdout, _, code = runMain(t, nil, "--config", warnProject(t, ws.URL, "strict: true\n"), "--tag", "R9", "require")
	want := "⚠ " + noBranch + "\nError: failed to require single: unknown producer or version (404)"
	if code == 0 || !strings.Contains(stdout, want) || !strings.HasSuffix(stdout, "\nError: strict: Sanshain warned: "+noBranch+"\n") {
		t.Errorf("expected the error, then the strict failure, got %d %q", code, stdout)
	}
}

// Trunk and a tag together warns and sends neither, for flags and env alike.
func TestWarnings_LocalTrunkAndTag(t *testing.T) {
	sources := map[string]struct {
		env  []string
		args []string
	}{
		"flags": {nil, []string{"--trunk", "--tag", "R1"}},
		"env":   {[]string{"SANSHAIN_TRUNK=true", "SANSHAIN_TAG=R1"}, nil},
	}
	for name, src := range sources {
		for _, command := range []string{"provide", "require", "catalog"} {
			ws := newWarnServer(t, http.StatusOK, recordedCatalog)
			args := append(append([]string{"--config", warnProject(t, ws.URL, "")}, src.args...), command)
			stdout, _, code := runMain(t, src.env, args...)
			if code != 0 || !strings.Contains(stdout, "⚠ "+neither+"\n") {
				t.Errorf("%s %s: expected the warning and exit 0, got %d %q", name, command, code, stdout)
			}
			seen := ws.seen()
			if len(seen) == 0 {
				t.Errorf("%s %s: the build must still run", name, command)
			}
			for _, r := range seen {
				_, trunk := r.body["trunk"]
				_, tag := r.body["tag"]
				if trunk || tag || strings.Contains(r.query, "trunk") || strings.Contains(r.query, "tag") {
					t.Errorf("%s %s: neither stream may be sent, got %s?%s %v", name, command, r.path, r.query, r.body)
				}
			}
		}
	}
}

// Under strict, trunk and a tag together fails before any request, even
// under best-effort.
func TestWarnings_LocalTrunkAndTagStrict(t *testing.T) {
	ws := newWarnServer(t, http.StatusOK, recordedCatalog)
	stdout, _, code := runMain(t, nil, "--config", warnProject(t, ws.URL, "strict: true\n"), "--best-effort", "--trunk", "--tag", "R1", "provide")
	if code == 0 || !strings.Contains(stdout, "Error: strict: "+neither+"\n") {
		t.Errorf("expected strict to fail, got %d %q", code, stdout)
	}
	if strings.Contains(stdout, "Sanshain warned") {
		t.Errorf("the client produced this warning itself, got %q", stdout)
	}
	if seen := ws.seen(); len(seen) != 0 {
		t.Errorf("no request may be sent, got %v", seen)
	}
}

// A catalog Sanshain could not read records nothing: a warning, exit 0.
func TestWarnings_CatalogNotRecorded(t *testing.T) {
	notRecorded := `{"system":"svc","trunk":false,"resources":[],"recorded":false}`
	reason := "catalog-info.yaml has no Component named 'svc'"

	ws := newWarnServer(t, http.StatusOK, notRecorded, reason)
	stdout, _, code := runMain(t, nil, "--config", warnProject(t, ws.URL, ""), "catalog")
	if code != 0 || !strings.Contains(stdout, "⚠ "+reason+"\n") || strings.Contains(stdout, "Catalog recorded") {
		t.Errorf("expected the reason as a warning, nothing recorded, exit 0, got %d %q", code, stdout)
	}

	ws = newWarnServer(t, http.StatusOK, notRecorded)
	cfgPath := warnProject(t, ws.URL, "")
	stdout, _, code = runMain(t, nil, "--config", cfgPath, "catalog")
	file := filepath.Join(filepath.Dir(cfgPath), "catalog-info.yaml")
	if code != 0 || !strings.Contains(stdout, "⚠ Sanshain did not record "+file+"\n") {
		t.Errorf("expected a warning naming the file, got %d %q", code, stdout)
	}

	ws = newWarnServer(t, http.StatusOK, notRecorded)
	stdout, _, code = runMain(t, nil, "--config", warnProject(t, ws.URL, "strict: true\n"), "catalog")
	if code == 0 || !strings.Contains(stdout, "Error: strict: Sanshain warned: Sanshain did not record ") {
		t.Errorf("expected strict to fail, got %d %q", code, stdout)
	}

	ws = newWarnServer(t, http.StatusOK, notRecorded, reason)
	stdout, _, code = runMain(t, nil, "--config", warnProject(t, ws.URL, "strict: true\n"), "catalog")
	if code == 0 || !strings.Contains(stdout, "Error: strict: Sanshain warned: "+reason+"\n") {
		t.Errorf("expected strict to fail with the header's reason, got %d %q", code, stdout)
	}
}
