package cache

import "testing"

func TestKeysIncludeFormat(t *testing.T) {
	if RequireKey("p", "1.0.0", "GET", "/a", "", "gen") == RequireKey("p", "1.0.0", "GET", "/a", "openapi", "gen") {
		t.Error("single-endpoint keys must differ per format")
	}
	if RequireBundleKey("p", "1.0.0", "", "gen") == RequireBundleKey("p", "1.0.0", "openapi", "gen") {
		t.Error("bundle keys must differ per format")
	}
}

func TestKeysIncludeTheOutputDirectory(t *testing.T) {
	if RequireKey("p", "1.0.0", "GET", "/a", "", "one") == RequireKey("p", "1.0.0", "GET", "/a", "", "two") {
		t.Error("single-endpoint keys must differ per output directory")
	}
	if RequireBundleKey("p", "1.0.0", "", "one") == RequireBundleKey("p", "1.0.0", "", "two") {
		t.Error("bundle keys must differ per output directory")
	}
	if got := RequireBundleKey("p", "1.0.0", "openapi", "gen/./api/"); got != "p|1.0.0|bundle|format=openapi|out=gen/api" {
		t.Errorf("unexpected key %q", got)
	}
}
