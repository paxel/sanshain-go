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
