package cache

import "testing"

func TestDecodeState_ReadsTheRequires(t *testing.T) {
	state := decodeState([]byte(`{"requires":{"p|1.0.0|bundle|out=gen":{"etag":"\"abc\"","last_fetched":"2026-11-18T10:00:00Z"}}}`))
	entry := state.Requires["p|1.0.0|bundle|out=gen"]
	if entry == nil || entry.Etag != `"abc"` || entry.LastFetched != "2026-11-18T10:00:00Z" {
		t.Errorf("unexpected entry %+v", entry)
	}
}

func TestDecodeState_WhatCannotBeReadIsAnEmptyState(t *testing.T) {
	for _, content := range []string{"", "not json", "[]", `{"requires":null}`, `{"requires":7}`} {
		if state := decodeState([]byte(content)); state.Requires == nil || len(state.Requires) != 0 {
			t.Errorf("expected an empty state for %q, got %+v", content, state)
		}
	}
}

func TestNewSanshainCache_ReadsWhatSaveWrote(t *testing.T) {
	dir := t.TempDir()
	first := NewSanshainCache(dir)
	if first.GetRequireEntry("key") != nil {
		t.Fatal("expected no entry in a new state")
	}
	first.UpdateRequireEntry("key", "etag-1")
	if err := first.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if entry := NewSanshainCache(dir).GetRequireEntry("key"); entry == nil || entry.Etag != "etag-1" {
		t.Errorf("expected the saved entry, got %+v", entry)
	}
}

// FuzzState reads the require state file as any content: never a panic, and
// always a state whose requires can be looked up and written.
func FuzzState(f *testing.F) {
	f.Add([]byte(`{"requires":{"p|1.0.0|GET|/a|out=gen":{"etag":"abc","last_fetched":"2026-11-18T10:00:00Z"}}}`))
	f.Add([]byte(`{"requires":{"key":null}}`))
	f.Add([]byte(`{"requires":null}`))
	f.Add([]byte(`{"requires":[1,2]}`))
	f.Add([]byte(""))
	f.Fuzz(func(t *testing.T, data []byte) {
		c := &SanshainCache{State: decodeState(data)}
		if c.State.Requires == nil {
			t.Fatal("expected a map of requires")
		}
		_ = c.GetRequireEntry("key")
		c.UpdateRequireEntry("key", "etag")
		if entry := c.GetRequireEntry("key"); entry == nil || entry.Etag != "etag" {
			t.Fatalf("expected the entry just written, got %+v", entry)
		}
	})
}
