package main

import (
	"path/filepath"
	"strings"
	"testing"
)

const catProvide = `
serviceName: svc
provide:
  file: api.yaml
`

func TestMain_UnknownFieldCatOnStderrAndContinues(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sanshain.yaml")
	writeFile(t, cfgPath, "sanshainUrl: http://127.0.0.1:1\nserviceName: svc\ncatalogInf: x\n")

	stdout, stderr, code := runMain(t, nil, "--config", cfgPath, "require")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stdout %q)", code, stdout)
	}
	want := "   /\\_/\\  ?\n  ( O_o )   sanshain.yaml: unknown field 'catalogInf' - ignored\n ---U-U---\n"
	if stderr != want {
		t.Errorf("expected cat on stderr exactly once:\n%q\ngot:\n%q", want, stderr)
	}
	if strings.Contains(stdout, "O_o") {
		t.Errorf("cat must not go to stdout: %q", stdout)
	}
}

func TestMain_UnknownFieldStrictFailsEvenBestEffort(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sanshain.yaml")
	writeFile(t, cfgPath, "sanshainUrl: http://127.0.0.1:1\nstrict: true\n"+catProvide+"  fiel: x\n")

	_, stderr, code := runMain(t, []string{"SANSHAIN_BEST_EFFORT=true"}, "--config", cfgPath, "--best-effort", "provide")
	if code == 0 {
		t.Fatal("expected a non-zero exit under strict, even with best-effort")
	}
	if !strings.Contains(stderr, "sanshain.yaml: unknown field 'provide.fiel' - rejected (strict)") {
		t.Errorf("unexpected stderr %q", stderr)
	}
}

func TestMain_LegacyFieldStillErrorsBeforeCat(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sanshain.yaml")
	writeFile(t, cfgPath, "sanshainUrl: http://127.0.0.1:1\nserviceName: svc\nbranch: main\ncatalogInf: x\n")

	stdout, stderr, code := runMain(t, nil, "--config", cfgPath, "require")
	if code == 0 {
		t.Fatal("expected the legacy field to fail")
	}
	if !strings.Contains(stdout, "'branch' is no longer supported") {
		t.Errorf("expected the legacy rejection, got %q", stdout)
	}
	if stderr != "" {
		t.Errorf("no cat may precede the legacy error, got %q", stderr)
	}
}

// A config that fails validation falls back to an empty config under
// best-effort, and the cat still names the typo.
func TestMain_BestEffortFallbackStillShowsCat(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "sanshain.yaml")
	writeFile(t, cfgPath, "sanshainUrll: http://127.0.0.1:1\nserviceName: svc\n")

	stdout, stderr, code := runMain(t, nil, "--config", cfgPath, "--best-effort", "require")
	if code != 0 {
		t.Fatalf("expected exit 0 under best-effort, got %d: %s", code, stdout)
	}
	if !strings.Contains(stderr, "sanshain.yaml: unknown field 'sanshainUrll' - ignored") {
		t.Errorf("expected the cat on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Warning loading config: missing required field: sanshainUrl") {
		t.Errorf("expected the config warning after the cat, got %q", stdout)
	}
}

func TestMain_TypoShowsCatThenValidationError(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "sanshain.yaml")
	writeFile(t, cfgPath, "sanshainUrll: http://127.0.0.1:1\nserviceName: svc\n")

	stdout, stderr, code := runMain(t, nil, "--config", cfgPath, "require")
	if code == 0 {
		t.Fatal("expected the missing sanshainUrl to fail")
	}
	if !strings.Contains(stderr, "unknown field 'sanshainUrll' - ignored") {
		t.Errorf("expected the cat on stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "Error loading config: missing required field: sanshainUrl") {
		t.Errorf("expected the missing-field error, got %q", stdout)
	}
}
