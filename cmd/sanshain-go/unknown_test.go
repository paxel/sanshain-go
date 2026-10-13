package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paxel/sanshain-go/v2/internal/config"
)

func TestReportUnknownFields(t *testing.T) {
	t.Run("nothing unknown prints nothing", func(t *testing.T) {
		var out bytes.Buffer
		if err := reportUnknownFields(&out, &config.SanshainConfig{Strict: true}); err != nil {
			t.Fatal(err)
		}
		if out.Len() != 0 {
			t.Errorf("expected no output, got %q", out.String())
		}
	})

	t.Run("not strict warns and continues", func(t *testing.T) {
		var out bytes.Buffer
		if err := reportUnknownFields(&out, &config.SanshainConfig{UnknownFields: []string{"catalogInf"}}); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.Contains(out.String(), "unknown field 'catalogInf' - ignored") {
			t.Errorf("unexpected output %q", out.String())
		}
	})

	t.Run("strict fails", func(t *testing.T) {
		var out bytes.Buffer
		err := reportUnknownFields(&out, &config.SanshainConfig{Strict: true, UnknownFields: []string{"catalogInf"}})
		if err == nil {
			t.Fatal("expected strict to fail")
		}
		if !strings.Contains(out.String(), "unknown field 'catalogInf' - rejected (strict)") {
			t.Errorf("unexpected output %q", out.String())
		}
	})
}

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
