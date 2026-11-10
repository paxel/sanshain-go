package main

import (
	"net/http"
	"strings"
	"testing"
)

// providedStabilities runs provide against a fresh server and returns the
// stability of every provide body it received ("" for a body without one).
func providedStabilities(t *testing.T, extra string, env []string, args ...string) ([]string, string) {
	t.Helper()
	ws := newWarnServer(t, http.StatusOK, recordedCatalog)
	all := append([]string{"--config", warnProject(t, ws.URL, extra)}, args...)
	stdout, _, code := runMain(t, env, append(all, "provide")...)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout)
	}
	var got []string
	for _, r := range ws.seen() {
		if strings.HasPrefix(r.path, "/provide") {
			s, _ := r.body["stability"].(string)
			got = append(got, s)
		}
	}
	return got, stdout
}

// autoGa makes the trunk and tag builds GA, from flags and env alike; a
// build without a stream, or one switching GA off, stays a snapshot.
func TestMain_AutoGa(t *testing.T) {
	cases := []struct {
		name  string
		extra string
		env   []string
		args  []string
		want  string
	}{
		{"trunk flag", "autoGa: true\n", nil, []string{"--trunk"}, "ga"},
		{"tag env", "autoGa: true\n", []string{"SANSHAIN_TAG=release-1"}, nil, "ga"},
		{"no stream", "autoGa: true\n", nil, nil, "snapshot"},
		{"SANSHAIN_GA=false", "autoGa: true\n", []string{"SANSHAIN_GA=false"}, []string{"--trunk"}, "snapshot"},
		{"--ga=false", "autoGa: true\n", nil, []string{"--ga=false", "--trunk"}, "snapshot"},
		{"autoGa off", "autoGa: false\n", nil, []string{"--trunk"}, "snapshot"},
		{"--ga=false beats SANSHAIN_GA=true", "", []string{"SANSHAIN_GA=true"}, []string{"--ga=false"}, "snapshot"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, stdout := providedStabilities(t, tc.extra, tc.env, tc.args...)
			if len(got) != 1 || got[0] != tc.want {
				t.Errorf("expected one provide with stability %s, got %v", tc.want, got)
			}
			if !strings.Contains(stdout, "(stability: "+tc.want+")") {
				t.Errorf("expected the stability in the log, got %q", stdout)
			}
		})
	}
}

// A retire never carries a stability or a stream, autoGa or not.
func TestMain_AutoGaRetireSendsNoStability(t *testing.T) {
	ws := newWarnServer(t, http.StatusOK, recordedCatalog)
	extra := "autoGa: true\nprovides:\n  - apiType: asyncapi\n    retired: true\n"
	stdout, _, code := runMain(t, nil, "--config", warnProject(t, ws.URL, extra), "--trunk", "provide")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout)
	}
	retires := 0
	for _, r := range ws.seen() {
		if r.body["retired"] != true {
			continue
		}
		retires++
		for _, field := range []string{"stability", "trunk", "tag"} {
			if _, present := r.body[field]; present {
				t.Errorf("a retire must not send %s, got %v", field, r.body)
			}
		}
	}
	if retires != 1 {
		t.Errorf("expected one retire, got %d: %v", retires, ws.seen())
	}
}
