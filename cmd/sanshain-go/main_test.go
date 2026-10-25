package main

import "testing"

func TestResolveStability(t *testing.T) {
	t.Run("default is snapshot", func(t *testing.T) {
		t.Setenv("SANSHAIN_GA", "")
		if got := resolveStability(false); got != "snapshot" {
			t.Errorf("expected snapshot by default, got %s", got)
		}
	})

	t.Run("ga flag switches to ga", func(t *testing.T) {
		t.Setenv("SANSHAIN_GA", "")
		if got := resolveStability(true); got != "ga" {
			t.Errorf("expected ga with --ga flag, got %s", got)
		}
	})

	t.Run("SANSHAIN_GA=true switches to ga", func(t *testing.T) {
		t.Setenv("SANSHAIN_GA", "true")
		if got := resolveStability(false); got != "ga" {
			t.Errorf("expected ga with SANSHAIN_GA=true, got %s", got)
		}
	})

	t.Run("SANSHAIN_GA=false stays snapshot", func(t *testing.T) {
		t.Setenv("SANSHAIN_GA", "false")
		if got := resolveStability(false); got != "snapshot" {
			t.Errorf("expected snapshot with SANSHAIN_GA=false, got %s", got)
		}
	})

	t.Run("flag wins even when env is false", func(t *testing.T) {
		t.Setenv("SANSHAIN_GA", "false")
		if got := resolveStability(true); got != "ga" {
			t.Errorf("expected ga when --ga is set regardless of env, got %s", got)
		}
	})
}

func TestResolveStability_EnvSwitchSpellings(t *testing.T) {
	for value, want := range map[string]string{"1": "ga", "TRUE": "ga", " true ": "ga", "yes": "snapshot"} {
		t.Setenv("SANSHAIN_GA", value)
		if got := resolveStability(false); got != want {
			t.Errorf("SANSHAIN_GA=%q: got %s, want %s", value, got, want)
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
	if code != 0 || stdout != "sanshain-go 2.4.0\n" {
		t.Errorf("expected 'sanshain-go 2.4.0', got %d %q", code, stdout)
	}
}
