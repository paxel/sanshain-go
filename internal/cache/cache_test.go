package cache

import "testing"

func TestKeysIncludeFormat(t *testing.T) {
	if RequireKey("p", "1.0.0", "GET", "/a", "") == RequireKey("p", "1.0.0", "GET", "/a", "openapi") {
		t.Error("single-endpoint keys must differ per format")
	}
	if RequireBundleKey("p", "1.0.0", "") == RequireBundleKey("p", "1.0.0", "openapi") {
		t.Error("bundle keys must differ per format")
	}
	if got := RequireBundleKey("p", "1.0.0", ""); got != "p|1.0.0|bundle" {
		t.Errorf("keys without format stay unchanged, got %q", got)
	}
}
