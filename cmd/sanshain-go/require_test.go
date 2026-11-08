package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paxel/sanshain-go/v2/internal/api"
	"github.com/paxel/sanshain-go/v2/internal/cache"
	"github.com/paxel/sanshain-go/v2/internal/config"
)

// requireProject writes a sanshain.yaml with the given requires block and
// returns its path and folder.
func requireProject(t *testing.T, url, requires string) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "conf")
	cfgPath := filepath.Join(dir, "sanshain.yaml")
	writeFile(t, cfgPath, "sanshainUrl: "+url+"\nserviceName: svc\nrequires:\n"+requires)
	return cfgPath, dir
}

// apiType grpc is proto: the single route is /require/grpc, the bundle says
// proto, and both files are saved as .proto.
func TestRequire_GrpcIsProto(t *testing.T) {
	ws := newWarnServer(t, http.StatusOK, "")
	cfgPath, dir := requireProject(t, ws.URL, `  - serviceName: single
    apiType: grpc
    version: 1.0.0
    outputDirectory: one
    endpoints:
      - method: SayHello
        path: helloworld.Greeter
  - serviceName: bundle
    apiType: grpc
    version: 1.0.0
    outputDirectory: two
    endpoints:
      - method: SayHello
        path: helloworld.Greeter
      - method: SayBye
        path: helloworld.Greeter
`)
	stdout, _, code := runMain(t, nil, "--config", cfgPath, "require")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout)
	}
	seen := ws.seen()
	if len(seen) != 2 || seen[0].path != "/require/grpc" || seen[1].path != "/require-bundle" {
		t.Fatalf("unexpected requests: %+v", seen)
	}
	if seen[1].body["api_type"] != "proto" {
		t.Errorf("the bundle must say api_type proto, got %v", seen[1].body["api_type"])
	}
	for _, file := range []string{"one/single.proto", "two/bundle_bundle.proto"} {
		if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
			t.Errorf("expected %s: %v", file, err)
		}
	}
}

// etagServer answers a require with ETag "e1", and with 304 when the request
// carries it as If-None-Match.
func etagServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"e1"`)
		if r.Header.Get("If-None-Match") == `"e1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte(openapiDoc))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The same endpoint required into two directories is written to both, and a
// deleted output file is fetched again instead of being skipped on a 304.
func TestRequire_EtagPerOutputDirectoryAndLostFile(t *testing.T) {
	srv := etagServer(t)
	cfgPath, dir := requireProject(t, srv.URL, `  - serviceName: users
    version: 1.0.0
    outputDirectory: one
    endpoints:
      - method: GET
        path: /x
  - serviceName: users
    version: 1.0.0
    outputDirectory: two
    endpoints:
      - method: GET
        path: /x
`)
	cfg, err := config.LoadConfig(cfgPath, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	sc := cache.NewSanshainCache(t.TempDir())
	client := api.NewSanshainClient(srv.URL, "", false)
	one, two := filepath.Join(dir, "one", "users.yaml"), filepath.Join(dir, "two", "users.yaml")

	if err := handleRequire(client, cfg, sc, api.Stream{}, nil); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{one, two} {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("expected %s written: %v", file, err)
		}
	}

	if err := os.Remove(two); err != nil {
		t.Fatal(err)
	}
	if err := handleRequire(client, cfg, sc, api.Stream{}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(two); err != nil {
		t.Errorf("expected the deleted file fetched again: %v", err)
	}
}
