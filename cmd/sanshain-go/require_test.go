package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
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
