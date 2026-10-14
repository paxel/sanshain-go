package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/paxel/sanshain-go/v2/internal/api"
	"github.com/paxel/sanshain-go/v2/internal/cache"
	"github.com/paxel/sanshain-go/v2/internal/config"
)

// requireServer answers every require with body and records each request's
// path, raw query and If-None-Match; GET /version answers version (404 when
// empty).
type requireServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []*http.Request
}

func newRequireServer(t *testing.T, body, etag, version string) *requireServer {
	t.Helper()
	rs := &requireServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			if version == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`{"version":"` + version + `"}`))
			return
		}
		rs.mu.Lock()
		rs.requests = append(rs.requests, r.Clone(r.Context()))
		rs.mu.Unlock()
		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(rs.Close)
	return rs
}

const openapiDoc = "openapi: 3.0.3\ninfo:\n  title: events\n  version: 1.0.0\n"
const asyncapiDoc = "asyncapi: 3.0.0\ninfo:\n  title: events\n  version: 1.0.0\n"

func formatProject(t *testing.T, url, entry string) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "conf")
	cfgPath := filepath.Join(dir, "sanshain.yaml")
	writeFile(t, cfgPath, "sanshainUrl: "+url+"\nserviceName: svc\nrequires:\n"+entry)
	return cfgPath, dir
}

const singleEntry = `  - serviceName: events
    apiType: asyncapi
    format: openapi
    version: 1.0.0
    outputDirectory: gen
    endpoints:
      - method: SEND
        path: /orders
`

const bundleEntry = `  - serviceName: events
    apiType: asyncapi
    format: openapi
    version: 1.0.0
    outputDirectory: gen
    endpoints:
      - method: SEND
        path: /orders
      - method: SEND
        path: /payments
`

func TestFormat_QueryParamSent(t *testing.T) {
	cases := []struct {
		name, entry, path, file string
	}{
		{"single", singleEntry, "/require/asyncapi", "events.yaml"},
		{"bundle", bundleEntry, "/require-bundle", "events_bundle.yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := newRequireServer(t, openapiDoc, "", "2.4.0")
			cfgPath, dir := formatProject(t, rs.URL, tc.entry)
			stdout, stderr, code := runMain(t, nil, "--config", cfgPath, "require")
			if code != 0 {
				t.Fatalf("expected exit 0, got %d: %s", code, stdout)
			}
			if stderr != "" {
				t.Errorf("format is a known field, expected no cat, got %q", stderr)
			}
			if len(rs.requests) != 1 {
				t.Fatalf("expected one request, got %d", len(rs.requests))
			}
			r := rs.requests[0]
			if r.URL.Path != tc.path || r.URL.Query().Get("format") != "openapi" {
				t.Errorf("expected %s with format=openapi, got %s?%s", tc.path, r.URL.Path, r.URL.RawQuery)
			}
			got, err := os.ReadFile(filepath.Join(dir, "gen", tc.file))
			if err != nil || string(got) != openapiDoc {
				t.Errorf("expected the OpenAPI document in %s, got %q, %v", tc.file, got, err)
			}
		})
	}
}

func TestFormat_NotSentWithoutFormat(t *testing.T) {
	for _, entry := range []string{singleEntry, bundleEntry} {
		rs := newRequireServer(t, asyncapiDoc, "", "2.4.0")
		cfgPath, _ := formatProject(t, rs.URL, strings.Replace(entry, "    format: openapi\n", "", 1))
		stdout, _, code := runMain(t, nil, "--config", cfgPath, "require")
		if code != 0 {
			t.Fatalf("expected exit 0, got %d: %s", code, stdout)
		}
		if len(rs.requests) != 1 || rs.requests[0].URL.Query().Has("format") {
			t.Errorf("format must not be sent, got %v", rs.requests[0].URL.RawQuery)
		}
	}
}

func TestFormat_InvalidCombinationRejectedBeforeAnyCall(t *testing.T) {
	cases := map[string]string{
		"wrong apiType": strings.Replace(singleEntry, "apiType: asyncapi", "apiType: openapi", 1),
		"no apiType":    strings.Replace(singleEntry, "    apiType: asyncapi\n", "", 1),
		"unknown value": strings.Replace(singleEntry, "format: openapi", "format: json", 1),
	}
	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			rs := newRequireServer(t, openapiDoc, "", "2.4.0")
			cfgPath, _ := formatProject(t, rs.URL, entry)
			stdout, _, code := runMain(t, nil, "--config", cfgPath, "require")
			if code == 0 {
				t.Fatal("expected a configuration error")
			}
			if !strings.Contains(stdout, "requires[0] (events): format") ||
				!strings.Contains(stdout, "'apiType: asyncapi' with 'format: openapi'") {
				t.Errorf("expected the entry and the allowed combination named, got %q", stdout)
			}
			if len(rs.requests) != 0 {
				t.Errorf("no call may be made, got %d", len(rs.requests))
			}
		})
	}
}

func TestFormat_NonOpenAPIAnswerNeedsService240(t *testing.T) {
	cases := []struct {
		version, want string
	}{
		{"2.3.0", "format: openapi needs Sanshain Service 2.4.0 or later (server is 2.3.0)"},
		{"", "format: openapi needs Sanshain Service 2.4.0 or later (server version unknown)"},
	}
	for _, tc := range cases {
		for _, entry := range []string{singleEntry, bundleEntry} {
			rs := newRequireServer(t, asyncapiDoc, "", tc.version)
			cfgPath, dir := formatProject(t, rs.URL, entry)
			stdout, _, code := runMain(t, nil, "--config", cfgPath, "require")
			if code == 0 || !strings.Contains(stdout, tc.want) {
				t.Errorf("expected failure %q, got %d %q", tc.want, code, stdout)
			}
			if _, err := os.Stat(filepath.Join(dir, "gen")); !os.IsNotExist(err) {
				t.Errorf("nothing may be written, got %v", err)
			}
		}
	}
}

func TestFormat_BestEffortWarnsAndWritesNothing(t *testing.T) {
	rs := newRequireServer(t, asyncapiDoc, "", "2.3.0")
	cfgPath, dir := formatProject(t, rs.URL, singleEntry)
	stdout, _, code := runMain(t, nil, "--config", cfgPath, "--best-effort", "require")
	if code != 0 {
		t.Fatalf("expected exit 0 under best-effort, got %d", code)
	}
	if !strings.Contains(stdout, "Warning: failed to require events: format: openapi needs Sanshain Service 2.4.0 or later (server is 2.3.0)") {
		t.Errorf("expected a warning, got %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "gen")); !os.IsNotExist(err) {
		t.Errorf("nothing may be written, got %v", err)
	}
}

// The same endpoint required as AsyncAPI and as OpenAPI keeps two ETags.
func TestFormat_EtagKeyedPerFormat(t *testing.T) {
	rs := newRequireServer(t, openapiDoc, `"e-openapi"`, "2.4.0")
	cfgPath, _ := formatProject(t, rs.URL, singleEntry)
	cfg, err := config.LoadConfig(cfgPath, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	sc := cache.NewSanshainCache(t.TempDir())
	client := api.NewSanshainClient(rs.URL, "", false)

	if err := handleRequire(client, cfg, sc, api.Stream{}); err != nil {
		t.Fatal(err)
	}
	if err := handleRequire(client, cfg, sc, api.Stream{}); err != nil {
		t.Fatal(err)
	}
	cfg.Requires[0].Format = ""
	cfg.Requires[0].OutputDirectory = filepath.Join(t.TempDir(), "async")
	if err := handleRequire(client, cfg, sc, api.Stream{}); err != nil {
		t.Fatal(err)
	}

	if len(rs.requests) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(rs.requests))
	}
	if got := rs.requests[0].Header.Get("If-None-Match"); got != "" {
		t.Errorf("first request must send no ETag, got %q", got)
	}
	if got := rs.requests[1].Header.Get("If-None-Match"); got != `"e-openapi"` {
		t.Errorf("the openapi format must reuse its ETag, got %q", got)
	}
	if got := rs.requests[2].Header.Get("If-None-Match"); got != "" {
		t.Errorf("the asyncapi format must not share the openapi ETag, got %q", got)
	}
}

// A pre-2.4.0 service refuses the single-endpoint format parameter with 400.
func TestFormat_OldServerRejectsSingleRouteWith400(t *testing.T) {
	cases := []struct {
		name, version, errBody, want string
	}{
		{"old server", "2.3.0", "unknown field `format`, expected one of `consumername`", "format: openapi needs Sanshain Service 2.4.0 or later (server is 2.3.0)"},
		{"version unknown", "", "unknown field `format`", "format: openapi needs Sanshain Service 2.4.0 or later (server version unknown)"},
		{"current server keeps its message", "2.4.0", "cannot convert channel /orders: Avro payloads are not supported for format", "cannot convert channel /orders: Avro payloads are not supported for format"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/version" {
					if tc.version == "" {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					_, _ = w.Write([]byte(`{"version":"` + tc.version + `"}`))
					return
				}
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"` + tc.errBody + `"}`))
			}))
			defer ts.Close()
			cfgPath, dir := formatProject(t, ts.URL, singleEntry)

			stdout, _, code := runMain(t, nil, "--config", cfgPath, "require")
			if code == 0 || !strings.Contains(stdout, tc.want) {
				t.Errorf("expected failure %q, got %d %q", tc.want, code, stdout)
			}
			if tc.version == "2.4.0" && strings.Contains(stdout, "needs Sanshain Service 2.4.0") {
				t.Errorf("a 2.4.0 server's 400 must pass through, got %q", stdout)
			}

			stdout, _, code = runMain(t, nil, "--config", cfgPath, "--best-effort", "require")
			if code != 0 || !strings.Contains(stdout, "Warning: failed to require events: ") || !strings.Contains(stdout, tc.want) {
				t.Errorf("expected a best-effort warning with %q, got %d %q", tc.want, code, stdout)
			}
			if _, err := os.Stat(filepath.Join(dir, "gen")); !os.IsNotExist(err) {
				t.Errorf("nothing may be written, got %v", err)
			}
		})
	}
}
