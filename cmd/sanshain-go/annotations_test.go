package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const annotationsYaml = `annotations:
  - scope: version
    name: requirements
    value: "http://solar.system/id-{{AREA}}/requirements/{{ID}}"
  - scope: system
    name: jenkins
    value: "https://jenkins.corp/job/payments"
`

var (
	requirementsJSON = map[string]any{"scope": "version", "name": "requirements", "value": "http://solar.system/id-7/requirements/42"}
	jenkinsJSON      = map[string]any{"scope": "system", "name": "jenkins", "value": "https://jenkins.corp/job/payments"}
)

const unfilledID = "annotation 'requirements' left out: {{ID}} is not set (--set ID=… or SANSHAIN_SET_ID)"

func TestSetFlags(t *testing.T) {
	sets := setFlags{}
	for _, arg := range []string{"A=x=y", "B=", "C=1", "C=2"} {
		if err := sets.Set(arg); err != nil {
			t.Fatalf("%q: %v", arg, err)
		}
	}
	want := setFlags{"A": "x=y", "B": "", "C": "2"}
	if !reflect.DeepEqual(sets, want) {
		t.Errorf("expected %v, got %v", want, sets)
	}
	for _, bad := range []string{"A", "=x", ""} {
		if err := sets.Set(bad); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}

// annotationsOf returns the annotations field of a request body, nil when absent.
func annotationsOf(r seenRequest) any {
	return r.body["annotations"]
}

// The filled annotations travel with every call: all of them on a provide,
// the system ones on require (query), bundle and catalog (body).
func TestMain_AnnotationsTravel(t *testing.T) {
	env := []string{"SANSHAIN_SET_ID=42"}
	for _, command := range []string{"provide", "require", "catalog"} {
		ws := newWarnServer(t, http.StatusOK, recordedCatalog)
		stdout, _, code := runMain(t, env, "--config", warnProject(t, ws.URL, annotationsYaml), "--set", "AREA=7", command)
		if code != 0 || strings.Contains(stdout, "left out") {
			t.Fatalf("%s: expected exit 0 and no warning, got %d %q", command, code, stdout)
		}
		if len(ws.seen()) == 0 {
			t.Errorf("%s: expected requests", command)
		}
		for _, r := range ws.seen() {
			switch {
			case strings.HasPrefix(r.path, "/provide"):
				if want := []any{requirementsJSON, jenkinsJSON}; !reflect.DeepEqual(annotationsOf(r), want) {
					t.Errorf("provide: expected %v, got %v", want, annotationsOf(r))
				}
			case r.path == "/require":
				q, _ := url.ParseQuery(r.query)
				var got any
				_ = json.Unmarshal([]byte(q.Get("annotations")), &got)
				if want := []any{jenkinsJSON}; !reflect.DeepEqual(got, want) {
					t.Errorf("require: expected %v on the query, got %q", want, q.Get("annotations"))
				}
			default: // bundle and catalog
				if want := []any{jenkinsJSON}; !reflect.DeepEqual(annotationsOf(r), want) {
					t.Errorf("%s: expected %v, got %v", r.path, want, annotationsOf(r))
				}
			}
		}
	}
}

// A retire never sends annotations; the provide beside it does.
func TestMain_RetireSendsNoAnnotations(t *testing.T) {
	ws := newWarnServer(t, http.StatusOK, recordedCatalog)
	extra := annotationsYaml + "provides:\n  - apiType: asyncapi\n    retired: true\n"
	stdout, _, code := runMain(t, []string{"SANSHAIN_SET_AREA=7", "SANSHAIN_SET_ID=42"}, "--config", warnProject(t, ws.URL, extra), "provide")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout)
	}
	seen := ws.seen()
	if len(seen) != 2 {
		t.Fatalf("expected a provide and a retire, got %v", seen)
	}
	for _, r := range seen {
		_, present := r.body["annotations"]
		if retire := r.body["retired"] == true; present == retire {
			t.Errorf("annotations on a provide only, got %v", r.body)
		}
	}
}

// --set splits at the first =, so the value may hold more.
func TestMain_SetValueWithEqualsSign(t *testing.T) {
	ws := newWarnServer(t, http.StatusOK, recordedCatalog)
	yaml := "annotations:\n  - scope: system\n    name: query\n    value: \"q={{Q}}\"\n"
	stdout, _, code := runMain(t, nil, "--config", warnProject(t, ws.URL, yaml), "--set", "Q=x=y", "catalog")
	want := []any{map[string]any{"scope": "system", "name": "query", "value": "q=x=y"}}
	if seen := ws.seen(); code != 0 || len(seen) != 1 || !reflect.DeepEqual(annotationsOf(seen[0]), want) {
		t.Errorf("expected %v, got %d %q %v", want, code, stdout, seen)
	}
}

// A --set without NAME= is a usage error before any request.
func TestMain_MalformedSet(t *testing.T) {
	ws := newWarnServer(t, http.StatusOK, recordedCatalog)
	_, stderr, code := runMain(t, nil, "--config", warnProject(t, ws.URL, annotationsYaml), "--set", "AREA", "provide")
	if code != 2 || !strings.Contains(stderr, `expected NAME=value, got "AREA"`) || len(ws.seen()) != 0 {
		t.Errorf("expected a usage error and no request, got %d %q %v", code, stderr, ws.seen())
	}
}

// An unfilled entry is left out with a warning; the others are still sent.
func TestMain_UnfilledAnnotationLeftOut(t *testing.T) {
	ws := newWarnServer(t, http.StatusOK, recordedCatalog)
	stdout, _, code := runMain(t, nil, "--config", warnProject(t, ws.URL, annotationsYaml), "--set", "AREA=7", "--set", "ID=", "provide")
	if code != 0 || !strings.Contains(stdout, "⚠ "+unfilledID+"\n") {
		t.Fatalf("expected the warning and exit 0, got %d %q", code, stdout)
	}
	if seen := ws.seen(); len(seen) != 1 || !reflect.DeepEqual(annotationsOf(seen[0]), []any{jenkinsJSON}) {
		t.Errorf("expected only jenkins sent, got %v", seen)
	}
}

// Under strict the unfilled warning fails before any request, also under
// best-effort.
func TestMain_UnfilledAnnotationStrict(t *testing.T) {
	for _, extra := range [][]string{nil, {"--best-effort"}} {
		ws := newWarnServer(t, http.StatusOK, recordedCatalog)
		args := append([]string{"--config", warnProject(t, ws.URL, "strict: true\n"+annotationsYaml), "--set", "AREA=7"}, extra...)
		stdout, _, code := runMain(t, nil, append(args, "provide")...)
		if code != 1 || !strings.Contains(stdout, "⚠ "+unfilledID+"\nError: strict: "+unfilledID+"\n") {
			t.Errorf("%v: expected strict to fail, got %d %q", extra, code, stdout)
		}
		if seen := ws.seen(); len(seen) != 0 {
			t.Errorf("%v: no request may be sent, got %v", extra, seen)
		}
	}
}

// oldService answers like a service before 2.13.0: annotations on the query
// are a 400, in the body a 422, each naming the field.
func oldService(t *testing.T) (*httptest.Server, func() []seenRequest) {
	t.Helper()
	var mu sync.Mutex
	var seen []seenRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			_, _ = w.Write([]byte(`{"version":"2.12.0"}`))
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		seen = append(seen, seenRequest{r.URL.Path, r.URL.RawQuery, body})
		mu.Unlock()
		if r.URL.Query().Has("annotations") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Failed to deserialize query string: unknown field `annotations`, expected one of `consumername`, `producername`, `version`, `path`, `method`, `dry_run`, `trunk`, `tag`, `format`"))
			return
		}
		if _, present := body["annotations"]; present {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte("Failed to deserialize the JSON body into the target type: unknown field `annotations`, expected one of `consumername`, `producername`, `version`, `api_type`, `endpoints`, `dry_run`, `trunk`, `tag`"))
			return
		}
		_, _ = w.Write([]byte(openapiDoc))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []seenRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]seenRequest(nil), seen...)
	}
}

// After the first refusal the command sends no more annotations and warns
// once; strict does not fail on that warning.
func TestMain_OldServiceWarnsOnce(t *testing.T) {
	srv, seen := oldService(t)
	stdout, stderr, code := runMain(t, nil, "--config", warnProject(t, srv.URL, "strict: true\n"+annotationsYaml), "--set", "AREA=7", "--set", "ID=42", "require")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s %s", code, stdout, stderr)
	}
	if got := strings.Count(stderr, "⚠ Sanshain Service before 2.13.0 does not take annotations; sending without them\n"); got != 1 {
		t.Errorf("expected the warning once, got %d in %q", got, stderr)
	}
	calls := seen()
	if len(calls) != 3 {
		t.Fatalf("expected the single require twice, then the bundle once, got %v", calls)
	}
	if calls[2].path != "/require-bundle" || calls[2].body["annotations"] != nil {
		t.Errorf("expected the bundle without annotations, got %v", calls[2])
	}
}
