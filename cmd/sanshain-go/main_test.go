package main

import (
	"strings"
	"testing"

	"github.com/paxel/sanshain-go/v2/internal/api"
)

func ptr(b bool) *bool { return &b }

// The autoGa table every Sanshain client tests: autoGa, the declared stream,
// the --ga flag and SANSHAIN_GA, and the stability they resolve to.
func TestResolveStability_AutoGa(t *testing.T) {
	declared := func(trunk bool, tag string) bool {
		stream, warning := api.ResolveStream(trunk, tag, "", "")
		return streamDeclared(stream, warning)
	}
	trunk, tag, none, both := declared(true, ""), declared(false, "release-1"), declared(false, ""), declared(true, "release-1")
	cases := []struct {
		name     string
		autoGa   bool
		declared bool
		flag     *bool
		env      string
		want     string
	}{
		{"autoGa on trunk", true, trunk, nil, "", "ga"},
		{"autoGa on a tag", true, tag, nil, "", "ga"},
		{"autoGa without a stream", true, none, nil, "", "snapshot"},
		{"SANSHAIN_GA=false beats autoGa", true, trunk, nil, "false", "snapshot"},
		{"--ga=false beats autoGa", true, trunk, ptr(false), "", "snapshot"},
		{"autoGa off on trunk", false, trunk, nil, "", "snapshot"},
		{"SANSHAIN_GA=true without autoGa", false, none, nil, "true", "ga"},
		{"--ga beats SANSHAIN_GA=false", true, none, ptr(true), "false", "ga"},
		// Both trunk and a tag send neither, but still declared a stream.
		{"autoGa on trunk and a tag", true, both, nil, "", "ga"},
		{"--ga=false beats SANSHAIN_GA=true", false, none, ptr(false), "true", "snapshot"},
		{"nothing set is a snapshot", false, none, nil, "", "snapshot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveStability(tc.flag, tc.env, tc.autoGa, tc.declared); got != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

// A blank SANSHAIN_GA is unset; any other value is read like every switch.
func TestResolveStability_EnvSwitchSpellings(t *testing.T) {
	for value, want := range map[string]string{"1": "ga", "TRUE": "ga", " true ": "ga", "yes": "snapshot", "0": "snapshot", "": "ga", " ": "ga"} {
		if got := resolveStability(nil, value, true, true); got != want {
			t.Errorf("SANSHAIN_GA=%q with autoGa on trunk: got %s, want %s", value, got, want)
		}
	}
}

func TestResolveBestEffort(t *testing.T) {
	cases := []struct {
		name      string
		env       string
		flagValue bool
		yamlValue bool
		want      bool
	}{
		{"unset env keeps yaml off", "", false, false, false},
		{"unset env keeps yaml on", "", false, true, true},
		{"flag wins", "false", true, false, true},
		{"1 is on", "1", false, false, true},
		{"TRUE is on", "TRUE", false, false, true},
		{"padded true is on", " true ", false, false, true},
		{"yes is off and overrides yaml", "yes", false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SANSHAIN_BEST_EFFORT", tc.env)
			if got := resolveBestEffort(tc.flagValue, tc.yamlValue); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMain_VersionFlag(t *testing.T) {
	stdout, _, code := runMain(t, nil, "--version")
	if code != 0 || stdout != "sanshain-go 2.5.0\n" {
		t.Errorf("expected 'sanshain-go 2.5.0', got %d %q", code, stdout)
	}
}

// The usage names what best effort turns into warnings.
func TestMain_BestEffortHelp(t *testing.T) {
	_, stderr, code := runMain(t, nil)
	if code != 1 || !strings.Contains(stderr, "turn failed calls, and a missing or broken sanshain.yaml, into warnings and exit 0") {
		t.Errorf("expected the best-effort help in the usage, got %d %q", code, stderr)
	}
}
