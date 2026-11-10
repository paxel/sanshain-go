package api

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

func noEnv(string) string { return "" }

func one(value string) []Annotation {
	return []Annotation{{Scope: "system", Name: "a", Value: value}}
}

// The placeholder table every Sanshain client tests.
func TestFillAnnotations_Placeholders(t *testing.T) {
	cases := []struct {
		name  string
		value string
		sets  map[string]string
		env   map[string]string
		want  string // "" means left out
	}{
		{"flag and env fill different placeholders", "x-{{A}}-{{B}}", map[string]string{"A": "1"}, map[string]string{"SANSHAIN_SET_B": "2"}, "x-1-2"},
		{"the flag beats the env", "{{A}}", map[string]string{"A": "f"}, map[string]string{"SANSHAIN_SET_A": "e"}, "f"},
		{"a repeated placeholder is filled every time", "{{A}}/{{A}}", map[string]string{"A": "7"}, nil, "7/7"},
		{"an empty flag is unfilled", "{{A}}", map[string]string{"A": ""}, nil, ""},
		{"an empty flag does not fall through to the env", "{{A}}", map[string]string{"A": ""}, map[string]string{"SANSHAIN_SET_A": "e"}, ""},
		{"an empty env value is unfilled", "{{A}}", nil, map[string]string{"SANSHAIN_SET_A": ""}, ""},
		{"the env name is case-exact", "{{a}}", nil, map[string]string{"SANSHAIN_SET_A": "x"}, ""},
		{"a plain value is unchanged", "plain value", nil, nil, "plain value"},
		{"spaces inside the braces are literal", "{{ A }}", map[string]string{"A": "1", " A ": "3"}, nil, "{{ A }}"},
		{"a dash is literal", "{{a-b}}", map[string]string{"a-b": "2"}, nil, "{{a-b}}"},
		{"empty braces are literal", "{{}}", nil, nil, "{{}}"},
		{"a third brace stays text", "{{{A}}}", map[string]string{"A": "1"}, nil, "{1}"},
		{"a value with an equals sign stays whole", "{{A}}", map[string]string{"A": "x=y"}, nil, "x=y"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filled, warnings := FillAnnotations(one(tc.value), tc.sets, func(k string) string { return tc.env[k] })
			if tc.want == "" {
				if len(filled) != 0 || len(warnings) != 1 {
					t.Errorf("expected the entry left out with one warning, got %v %v", filled, warnings)
				}
				return
			}
			if len(warnings) != 0 || len(filled) != 1 || filled[0].Value != tc.want {
				t.Errorf("expected %q, got %v %v", tc.want, filled, warnings)
			}
		})
	}
}

var exampleAnnotations = []Annotation{
	{Scope: "version", Name: "requirements", Value: "http://solar.system/id-{{AREA}}/requirements/{{ID}}"},
	{Scope: "system", Name: "jenkins", Value: "https://jenkins.corp/job/payments"},
}

// An unfilled entry is left out with a warning naming it and its first
// unfilled placeholder; the others are kept.
func TestFillAnnotations_UnfilledEntryLeftOut(t *testing.T) {
	filled, warnings := FillAnnotations(exampleAnnotations, map[string]string{"AREA": "7"}, noEnv)
	want := []string{"annotation 'requirements' left out: {{ID}} is not set (--set ID=… or SANSHAIN_SET_ID)"}
	if !reflect.DeepEqual(warnings, want) {
		t.Errorf("expected %q, got %q", want, warnings)
	}
	if !reflect.DeepEqual(filled, exampleAnnotations[1:]) {
		t.Errorf("expected only jenkins, got %v", filled)
	}

	_, warnings = FillAnnotations(one("{{MISSING}}"), nil, noEnv)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "'a' left out: {{MISSING}} is not set (--set MISSING=… or SANSHAIN_SET_MISSING)") {
		t.Errorf("expected the warning to name the entry and MISSING, got %q", warnings)
	}

	_, warnings = FillAnnotations(one("{{A}}{{B}}{{C}}"), map[string]string{"A": "1"}, noEnv)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "{{B}} is not set") {
		t.Errorf("expected the first unfilled placeholder named, got %q", warnings)
	}
}

func TestFillAnnotations_KeepsScopeNameAndOrder(t *testing.T) {
	env := func(k string) string { return map[string]string{"SANSHAIN_SET_ID": "42"}[k] }
	filled, warnings := FillAnnotations(exampleAnnotations, map[string]string{"AREA": "7"}, env)
	want := []Annotation{
		{Scope: "version", Name: "requirements", Value: "http://solar.system/id-7/requirements/42"},
		exampleAnnotations[1],
	}
	if len(warnings) != 0 || !reflect.DeepEqual(filled, want) {
		t.Errorf("expected %v, got %v %v", want, filled, warnings)
	}
}

func TestAnnotationsFieldRefused_OnlyTheExactText(t *testing.T) {
	refused := []string{
		"Failed to deserialize the JSON body into the target type: unknown field `annotations`, expected one of `producername`, `openapi_yaml`, `stability`",
		"Failed to deserialize query string: unknown field `annotations`, expected one of `consumername`, `producername`",
	}
	for _, body := range refused {
		if !annotationsFieldRefused(body) {
			t.Errorf("expected a refusal: %q", body)
		}
	}
	// A 2.13 service refusing something else lists annotations as expected.
	if annotationsFieldRefused("Failed to deserialize the JSON body into the target type: unknown field `colour`, expected one of `producername`, `annotations`, `build_url`") {
		t.Error("annotations only listed as expected is no refusal")
	}
	// The refusal of a 2.9–2.12 service lists the origin fields as expected.
	if originFieldRefused(provideRefusal) {
		t.Error("origin fields only listed as expected are no origin refusal")
	}
}

// What a 2.9–2.12 service answers to a provide carrying annotations.
const provideRefusal = "Failed to deserialize the JSON body into the target type: unknown field `annotations`, expected one of `producername`, `openapi_yaml`, `stability`, `dry_run`, `trunk`, `tag`, `build_url`, `change_url`, `tickets`, `note` at line 1 column 180"

// recorder answers like a service that refuses the fields listed in refuses,
// the first present one per answer, with status; GET /version answers 2.12.0.
type recorder struct {
	*httptest.Server
	mu       sync.Mutex
	bodies   []map[string]any
	queries  []string
	refuses  []string
	status   int
	okAnswer string
}

func newRecorder(t *testing.T, status int, okAnswer string, refuses ...string) *recorder {
	t.Helper()
	rec := &recorder{refuses: refuses, status: status, okAnswer: okAnswer}
	rec.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			serveVersion(w, "2.12.0")
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		rec.mu.Lock()
		rec.bodies = append(rec.bodies, body)
		rec.queries = append(rec.queries, r.URL.RawQuery)
		rec.mu.Unlock()
		for _, field := range rec.refuses {
			_, inBody := body[field]
			if inBody || r.URL.Query().Has(field) {
				w.WriteHeader(rec.status)
				_, _ = w.Write([]byte("Failed to deserialize the JSON body into the target type: unknown field `" + field + "`, expected one of `producername`, `openapi_yaml`, `stability`, `dry_run`, `trunk`, `tag`"))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(rec.okAnswer))
	}))
	t.Cleanup(rec.Close)
	return rec
}

const provideAnswer = `{"version":"1.0.0","stability":"snapshot","content_hash":"h","changes":{}}`

func filledExample() []Annotation {
	return []Annotation{
		{Scope: "version", Name: "requirements", Value: "http://solar.system/id-7/requirements/42"},
		{Scope: "system", Name: "jenkins", Value: "https://jenkins.corp/job/payments"},
	}
}

func asJSON(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

// A provide sends every annotation, both scopes, for every API type; none
// means no field at all.
func TestProvide_SendsEveryAnnotation(t *testing.T) {
	rec := newRecorder(t, 0, provideAnswer)
	c := NewSanshainClient(rec.URL, "", false)
	all := filledExample()
	_, err1 := c.Provide(ProvidePayload{ProducerName: "p", OpenApiYaml: "x", Stability: "ga", Annotations: all}, false)
	_, err2 := c.ProvideAsyncApi(ProvideAsyncApiPayload{ProducerName: "p", AsyncApiYaml: "x", Stability: "ga", Annotations: all}, false)
	_, err3 := c.ProvideProto(ProvideProtoPayload{ProducerName: "p", ProtoContent: "x", Stability: "ga", Annotations: all}, false)
	_, err4 := c.Provide(ProvidePayload{ProducerName: "p", OpenApiYaml: "x", Stability: "ga"}, false)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		t.Fatalf("provides failed: %v %v %v %v", err1, err2, err3, err4)
	}
	want := asJSON(t, all)
	for i, body := range rec.bodies[:3] {
		if !reflect.DeepEqual(body["annotations"], want) {
			t.Errorf("provide %d: expected %v, got %v", i, want, body["annotations"])
		}
	}
	if _, present := rec.bodies[3]["annotations"]; present {
		t.Errorf("no annotations means no field, got %v", rec.bodies[3])
	}
}

// A require sends the system annotations as a JSON array on the query.
func TestRequireWithEtag_SendsSystemAnnotationsOnTheQuery(t *testing.T) {
	rec := newRecorder(t, 0, "openapi: 3.0.3\n")
	c := NewSanshainClient(rec.URL, "", false)
	for _, apiType := range []string{"", "asyncapi", "proto"} {
		if _, err := c.RequireWithEtag("c", "p", "1.0.0", "/x", "GET", false, apiType, "", Stream{}, "", filledExample()); err != nil {
			t.Fatalf("%s: %v", apiType, err)
		}
	}
	_, _ = c.RequireWithEtag("c", "p", "1.0.0", "/x", "GET", false, "", "", Stream{}, "", filledExample()[:1])
	want := `[{"scope":"system","name":"jenkins","value":"https://jenkins.corp/job/payments"}]`
	for i, raw := range rec.queries[:3] {
		q := parseQuery(t, raw)
		if q.Get("annotations") != want {
			t.Errorf("require %d: expected %s, got %q", i, want, q.Get("annotations"))
		}
	}
	if parseQuery(t, rec.queries[3]).Has("annotations") {
		t.Errorf("only version annotations means no parameter, got %q", rec.queries[3])
	}
}

func parseQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	q, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

// A bundle and a catalog push send the system annotations in the body.
func TestBundleAndCatalog_SendSystemAnnotationsInTheBody(t *testing.T) {
	rec := newRecorder(t, 0, `{"system":"s","trunk":false,"resources":[],"recorded":true}`)
	c := NewSanshainClient(rec.URL, "", false)
	_, _ = c.RequireBundleWithEtag(RequireBundlePayload{ConsumerName: "c", ProducerName: "p", Version: "1.0.0", Annotations: filledExample()}, false, "")
	if _, err := c.PushCatalog("s", "x", false, false, filledExample()); err != nil {
		t.Fatal(err)
	}
	_, _ = c.RequireBundleWithEtag(RequireBundlePayload{ConsumerName: "c", ProducerName: "p", Version: "1.0.0", Annotations: filledExample()[:1]}, false, "")
	want := asJSON(t, filledExample()[1:])
	for i, body := range rec.bodies[:2] {
		if !reflect.DeepEqual(body["annotations"], want) {
			t.Errorf("call %d: expected %v, got %v", i, want, body["annotations"])
		}
	}
	if _, present := rec.bodies[2]["annotations"]; present {
		t.Errorf("only version annotations means no field, got %v", rec.bodies[2])
	}
}

// A service before 2.13.0 refuses annotations by name: the provide is
// repeated without them, keeping the origin, and the client sends none from
// then on.
func TestProvide_RepeatsWithoutAnnotationsWhenAnOldServiceRefusesThem(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		if _, present := body["annotations"]; present {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(provideRefusal))
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(provideAnswer))
	}))
	defer srv.Close()
	c := NewSanshainClient(srv.URL, "", false)
	payload := ProvidePayload{ProducerName: "p", OpenApiYaml: "x", Stability: "snapshot", Origin: Origin{BuildURL: "https://ci/1"}, Annotations: filledExample()}
	resp, err := c.Provide(payload, false)
	if err != nil || resp == nil || resp.Version != "1.0.0" {
		t.Fatalf("want the repeated provide to succeed, got %v, %v", resp, err)
	}
	if len(bodies) != 2 {
		t.Fatalf("want one repeat, got %d requests", len(bodies))
	}
	if _, present := bodies[1]["annotations"]; present || bodies[1]["build_url"] != "https://ci/1" {
		t.Errorf("want the repeat without annotations but with the origin, got %v", bodies[1])
	}
	if _, err := c.ProvideAsyncApi(ProvideAsyncApiPayload{ProducerName: "p", AsyncApiYaml: "x", Stability: "snapshot", Annotations: filledExample()}, false); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 3 {
		t.Errorf("after a refusal the client sends no annotations, got %d requests", len(bodies))
	}
}

// A 2.8 service refuses both the origin and the annotations, one per answer,
// in either order.
func TestProvide_DropsAnnotationsAndOriginInEitherOrder(t *testing.T) {
	for _, order := range [][]string{{"annotations", "note"}, {"note", "annotations"}} {
		rec := newRecorder(t, http.StatusUnprocessableEntity, provideAnswer, order...)
		c := NewSanshainClient(rec.URL, "", false)
		payload := ProvidePayload{ProducerName: "p", OpenApiYaml: "x", Stability: "snapshot", Origin: Origin{Note: "why"}, Annotations: filledExample()}
		if _, err := c.Provide(payload, false); err != nil {
			t.Fatalf("%v: want success, got %v", order, err)
		}
		if len(rec.bodies) != 3 {
			t.Fatalf("%v: want two repeats, got %d requests", order, len(rec.bodies))
		}
		last := rec.bodies[2]
		_, annotations := last["annotations"]
		_, note := last["note"]
		if annotations || note {
			t.Errorf("%v: want the last provide without both, got %v", order, last)
		}
	}
}

// Another unknown field is no fallback: the refusal fails the provide.
func TestProvide_OtherUnknownFieldIsNoFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			serveVersion(w, "2.13.0")
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte("Failed to deserialize the JSON body into the target type: unknown field `colour`, expected one of `producername`, `annotations`, `build_url`"))
	}))
	defer srv.Close()
	c := NewSanshainClient(srv.URL, "", false)
	_, err := c.Provide(ProvidePayload{ProducerName: "p", OpenApiYaml: "x", Origin: Origin{BuildURL: "https://ci/1"}, Annotations: filledExample()}, false)
	if err == nil || !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "colour") {
		t.Errorf("want the 422 to fail the provide, got %v", err)
	}
	if c.annotationsRefused {
		t.Error("annotations must not count as refused")
	}
}

// A service before 2.13.0 refuses the annotations query parameter with 400:
// the require is repeated without it, before the format refusal is checked
// (the refusal lists format among the expected fields).
func TestRequireWithEtag_RepeatsWithoutAnnotationsOn400(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			serveVersion(w, "2.12.0")
			return
		}
		queries = append(queries, r.URL.RawQuery)
		if r.URL.Query().Has("annotations") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Failed to deserialize query string: unknown field `annotations`, expected one of `consumername`, `producername`, `version`, `path`, `method`, `dry_run`, `trunk`, `tag`, `format`"))
			return
		}
		_, _ = w.Write([]byte("openapi: 3.0.3\n"))
	}))
	defer srv.Close()
	c := NewSanshainClient(srv.URL, "", false)
	result, err := c.RequireWithEtag("c", "p", "1.0.0", "/orders", "SEND", false, "asyncapi", "", Stream{}, "openapi", filledExample())
	if err != nil || result.Content != "openapi: 3.0.3\n" {
		t.Fatalf("want the repeated require to succeed, got %v, %v", result, err)
	}
	if len(queries) != 2 || strings.Contains(queries[1], "annotations") || !strings.Contains(queries[1], "format=openapi") {
		t.Errorf("want one repeat without annotations, got %q", queries)
	}
	_, _ = c.RequireWithEtag("c", "p", "1.0.0", "/x", "GET", false, "", "", Stream{}, "", filledExample())
	if len(queries) != 3 || strings.Contains(queries[2], "annotations") {
		t.Errorf("after a refusal the client sends no annotations, got %q", queries)
	}
}

// A bundle and a catalog push are repeated without annotations on the 422.
func TestBundleAndCatalog_RepeatWithoutAnnotationsOn422(t *testing.T) {
	rec := newRecorder(t, http.StatusUnprocessableEntity, "openapi: 3.0.3\n", "annotations")
	c := NewSanshainClient(rec.URL, "", false)
	result, err := c.RequireBundleWithEtag(RequireBundlePayload{ConsumerName: "c", ProducerName: "p", Version: "1.0.0", Annotations: filledExample()}, false, "")
	if err != nil || result.Content != "openapi: 3.0.3\n" || len(rec.bodies) != 2 {
		t.Fatalf("want the repeated bundle to succeed, got %v, %v, %d requests", result, err, len(rec.bodies))
	}

	rec = newRecorder(t, http.StatusUnprocessableEntity, `{"system":"s","trunk":false,"resources":[],"recorded":true}`, "annotations")
	c = NewSanshainClient(rec.URL, "", false)
	resp, err := c.PushCatalog("s", "x", true, false, filledExample())
	if err != nil || !resp.Recorded || len(rec.bodies) != 2 {
		t.Fatalf("want the repeated catalog push to succeed, got %v, %v, %d requests", resp, err, len(rec.bodies))
	}
	if _, present := rec.bodies[1]["annotations"]; present || rec.bodies[1]["trunk"] != true {
		t.Errorf("want the repeat without annotations, the rest kept, got %v", rec.bodies[1])
	}
}
