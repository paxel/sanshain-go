package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestResolveOrigin_NothingKnownIsZero(t *testing.T) {
	o := ResolveOrigin("", "", "", "", env(nil))
	if !o.IsZero() {
		t.Fatalf("want zero origin, got %+v", o)
	}
	b, _ := json.Marshal(ProvidePayload{ProducerName: "p", OpenApiYaml: "x", Stability: "snapshot"})
	if string(b) != `{"producername":"p","openapi_yaml":"x","stability":"snapshot"}` {
		t.Errorf("a zero origin must add no fields: %s", b)
	}
}

func TestResolveOrigin_FlagsBeatEnvironmentBeatsRunner(t *testing.T) {
	e := env(map[string]string{
		"SANSHAIN_BUILD_URL": "https://ci/env", "SANSHAIN_CHANGE_URL": "https://git/env",
		"SANSHAIN_TICKETS": "ENV-1", "SANSHAIN_NOTE": "env", "BUILD_URL": "https://jenkins/7",
	})
	o := ResolveOrigin(" https://ci/flag ", "https://git/flag", "A-1, A-2 ,A-1,", " flag ", e)
	want := Origin{BuildURL: "https://ci/flag", ChangeURL: "https://git/flag", Tickets: []string{"A-1", "A-2"}, Note: "flag"}
	if !reflect.DeepEqual(o, want) {
		t.Fatalf("got %+v, want %+v", o, want)
	}
	o = ResolveOrigin("", "", "", "", e)
	if o.BuildURL != "https://ci/env" || o.Note != "env" || !reflect.DeepEqual(o.Tickets, []string{"ENV-1"}) {
		t.Errorf("SANSHAIN_* must beat the runner: %+v", o)
	}
}

func TestResolveOrigin_Runners(t *testing.T) {
	gh := env(map[string]string{
		"GITHUB_SERVER_URL": "https://github.com", "GITHUB_REPOSITORY": "acme/orders",
		"GITHUB_RUN_ID": "123", "GITHUB_REF": "refs/pull/42/merge",
	})
	if o := ResolveOrigin("", "", "", "", gh); o.BuildURL != "https://github.com/acme/orders/actions/runs/123" ||
		o.ChangeURL != "https://github.com/acme/orders/pull/42" {
		t.Errorf("GitHub: %+v", o)
	}
	gl := env(map[string]string{
		"CI_JOB_URL": "https://gitlab/acme/orders/-/jobs/7", "CI_MERGE_REQUEST_PROJECT_URL": "https://gitlab/acme/orders",
		"CI_MERGE_REQUEST_IID": "12",
	})
	if o := ResolveOrigin("", "", "", "", gl); o.BuildURL != "https://gitlab/acme/orders/-/jobs/7" ||
		o.ChangeURL != "https://gitlab/acme/orders/-/merge_requests/12" {
		t.Errorf("GitLab: %+v", o)
	}
	jk := env(map[string]string{"BUILD_URL": "https://jenkins/job/7/", "CHANGE_URL": "https://bitbucket/pr/5"})
	if o := ResolveOrigin("", "", "", "", jk); o.BuildURL != "https://jenkins/job/7/" || o.ChangeURL != "https://bitbucket/pr/5" {
		t.Errorf("Jenkins: %+v", o)
	}
	push := env(map[string]string{"GITHUB_SERVER_URL": "https://github.com", "GITHUB_REPOSITORY": "a/b", "GITHUB_REF": "refs/heads/main"})
	if o := ResolveOrigin("", "", "", "", push); o.ChangeURL != "" {
		t.Errorf("a push build belongs to no change: %+v", o)
	}
}

func TestOrigin_IsFlattenedIntoEveryProvidePayload(t *testing.T) {
	o := Origin{ChangeURL: "https://git/pull/7", Tickets: []string{"ORD-7"}, Note: "why"}
	for _, p := range []any{
		ProvidePayload{ProducerName: "p", Origin: o},
		ProvideAsyncApiPayload{ProducerName: "p", Origin: o},
		ProvideProtoPayload{ProducerName: "p", Origin: o},
	} {
		b, _ := json.Marshal(p)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		if m["change_url"] != "https://git/pull/7" || m["note"] != "why" || m["tickets"] == nil {
			t.Errorf("origin not flattened: %s", b)
		}
		if _, ok := m["build_url"]; ok {
			t.Errorf("unset build_url must be omitted: %s", b)
		}
	}
}

// A service before 2.9.0 refuses the origin by name: the provide is repeated
// without it instead of failing the build.
func TestProvide_RepeatsWithoutTheOriginWhenAnOldServerRefusesIt(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if strings.Contains(string(b), `"note"`) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte("Failed to deserialize the JSON body into the target type: unknown field `note`"))
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"version":"1.0.0","stability":"snapshot","content_hash":"h","changes":{}}`))
	}))
	defer srv.Close()
	c := NewSanshainClient(srv.URL, "", false)
	resp, err := c.Provide(ProvidePayload{ProducerName: "p", OpenApiYaml: "x", Stability: "snapshot", Origin: Origin{Note: "why"}}, false)
	if err != nil || resp == nil || resp.Version != "1.0.0" {
		t.Fatalf("want the repeated provide to succeed, got %v, %v", resp, err)
	}
	if len(bodies) != 2 || strings.Contains(bodies[1], "note") {
		t.Fatalf("want a second request without the origin, got %q", bodies)
	}
}
