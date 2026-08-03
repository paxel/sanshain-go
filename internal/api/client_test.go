package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serveVersion answers GET /version like a Sanshain 2.0 instance, so the
// lazy wrong-server diagnosis keeps the original error.
func serveVersion(w http.ResponseWriter, instanceVersion string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"version":"` + instanceVersion + `","instance_id":"test"}`))
}

func TestSanshainClient_Provide_SendsStabilityAndNoBranchEraFields(t *testing.T) {
	var rawBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/provide" {
			t.Errorf("expected /provide, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &rawBody); err != nil {
			t.Fatalf("failed to decode provide body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"version":"1.4.0","stability":"snapshot","content_hash":"sha256:abc","changes":{"inserts":1,"updates":0,"deletes":0}}`))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	payload := ProvidePayload{
		ProducerName: "test",
		OpenApiYaml:  "openapi: 3.0.3",
		Stability:    "snapshot",
	}
	resp, err := client.Provide(payload, false)
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}
	if resp.Version != "1.4.0" {
		t.Errorf("expected version 1.4.0, got %s", resp.Version)
	}
	if resp.Stability != "snapshot" {
		t.Errorf("expected stability snapshot, got %s", resp.Stability)
	}

	if rawBody["producername"] != "test" {
		t.Errorf("expected producername test, got %v", rawBody["producername"])
	}
	if rawBody["stability"] != "snapshot" {
		t.Errorf("expected stability snapshot in payload, got %v", rawBody["stability"])
	}
	for _, forbidden := range []string{"branch", "base_version", "force", "timeout", "servicename"} {
		if _, present := rawBody[forbidden]; present {
			t.Errorf("branch-era field %q must not be sent, payload: %v", forbidden, rawBody)
		}
	}
}

func TestSanshainClient_Provide_GaStability(t *testing.T) {
	var rawBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &rawBody)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"version":"2.0.0","stability":"ga","content_hash":"sha256:abc","changes":{"inserts":0,"updates":0,"deletes":0}}`))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	_, err := client.Provide(ProvidePayload{ProducerName: "test", OpenApiYaml: "x", Stability: "ga"}, false)
	if err != nil {
		t.Fatalf("Provide failed: %v", err)
	}
	if rawBody["stability"] != "ga" {
		t.Errorf("expected stability ga in payload, got %v", rawBody["stability"])
	}
}

func TestSanshainClient_Provide_ConflictSurfacesProposedVersion(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			serveVersion(w, "2.0.0")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"version 1.2.0 is GA and immutable","proposed_version":"1.3.0"}`))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	_, err := client.Provide(ProvidePayload{ProducerName: "test", OpenApiYaml: "x", Stability: "ga"}, false)
	if err == nil {
		t.Fatal("expected 409 error")
	}

	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected ConflictError, got %T: %v", err, err)
	}
	if conflict.ProposedVersion != "1.3.0" {
		t.Errorf("expected proposed version 1.3.0, got %q", conflict.ProposedVersion)
	}
	if conflict.Message != "version 1.2.0 is GA and immutable" {
		t.Errorf("expected server message surfaced, got %q", conflict.Message)
	}
	if !strings.Contains(err.Error(), "1.3.0") {
		t.Errorf("expected error text to surface proposed version, got %q", err.Error())
	}
}

func TestSanshainClient_Require_SendsVersionAndNoBranchEraParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/require" {
			t.Errorf("expected /require, got %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("consumername") != "client" {
			t.Errorf("expected consumername client, got %s", q.Get("consumername"))
		}
		if q.Get("producername") != "service" {
			t.Errorf("expected producername service, got %s", q.Get("producername"))
		}
		if q.Get("version") != "1.2.0" {
			t.Errorf("expected version 1.2.0, got %s", q.Get("version"))
		}
		for _, forbidden := range []string{"branch", "timeout", "pull_from_branch", "source_protected_branch", "clientname", "servicename"} {
			if q.Has(forbidden) {
				t.Errorf("branch-era query param %q must not be sent", forbidden)
			}
		}
		_, _ = w.Write([]byte("openapi-content"))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	res, err := client.Require("client", "service", "1.2.0", "/path", "GET", false, "openapi")
	if err != nil {
		t.Fatalf("Require failed: %v", err)
	}
	if res != "openapi-content" {
		t.Errorf("expected openapi-content, got %s", res)
	}
}

func TestSanshainClient_Require_NotFoundIsDistinct(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			serveVersion(w, "2.0.0")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"no version 9.9.9 for producer service"}`))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	_, err := client.Require("client", "service", "9.9.9", "/path", "GET", false, "openapi")
	if err == nil {
		t.Fatal("expected 404 error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "404") || !strings.Contains(msg, "service@9.9.9") {
		t.Errorf("expected distinct 404 message naming the pin, got %q", msg)
	}
	if !strings.Contains(msg, "GET /producers/service/versions") {
		t.Errorf("expected hint to list available versions, got %q", msg)
	}
}

func TestSanshainClient_Require_GoneIsDistinct(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			serveVersion(w, "2.0.0")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"error":"GET /gone is not part of version 1.2.0"}`))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	_, err := client.Require("client", "service", "1.2.0", "/gone", "GET", false, "openapi")
	if err == nil {
		t.Fatal("expected 410 error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "410") || !strings.Contains(msg, "deliberately lacks") {
		t.Errorf("expected distinct 410 message, got %q", msg)
	}
	if !strings.Contains(msg, "GET /gone is not part of version 1.2.0") {
		t.Errorf("expected server message surfaced, got %q", msg)
	}
}

func TestSanshainClient_RequireBundle_SendsVersionPayload(t *testing.T) {
	var rawBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/require-bundle" {
			t.Errorf("expected /require-bundle, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &rawBody); err != nil {
			t.Fatalf("failed to decode bundle body: %v", err)
		}
		_, _ = w.Write([]byte("bundle-content"))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	payload := RequireBundlePayload{
		ConsumerName: "client",
		ProducerName: "service",
		Version:      "1.2.0",
		Endpoints:    []RequireBundleEndpoint{{Path: "/path", Method: "GET"}},
	}
	res, err := client.RequireBundle(payload, false)
	if err != nil {
		t.Fatalf("RequireBundle failed: %v", err)
	}
	if res != "bundle-content" {
		t.Errorf("expected bundle-content, got %s", res)
	}

	if rawBody["consumername"] != "client" || rawBody["producername"] != "service" {
		t.Errorf("expected consumername/producername, got %v", rawBody)
	}
	if rawBody["version"] != "1.2.0" {
		t.Errorf("expected version 1.2.0 in payload, got %v", rawBody["version"])
	}
	for _, forbidden := range []string{"branch", "timeout", "clientname", "servicename"} {
		if _, present := rawBody[forbidden]; present {
			t.Errorf("branch-era field %q must not be sent, payload: %v", forbidden, rawBody)
		}
	}
}

func TestSanshainClient_LazyWrongServerDiagnosis(t *testing.T) {
	versionCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			versionCalls++
			serveVersion(w, "1.7.3")
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`unknown field 'stability'`))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	_, err := client.Provide(ProvidePayload{ProducerName: "test", OpenApiYaml: "x", Stability: "snapshot"}, false)
	if err == nil {
		t.Fatal("expected error against pre-2.0 server")
	}
	expected := "Sanshain server at " + ts.URL + " is 1.7.3; this client requires Sanshain 2.x — upgrade the server"
	if err.Error() != expected {
		t.Errorf("expected wrong-server diagnosis %q, got %q", expected, err.Error())
	}
	if versionCalls != 1 {
		t.Errorf("expected exactly one lazy GET /version, got %d", versionCalls)
	}

	// A second failure must reuse the memoized diagnosis (no second GET /version).
	_, err = client.Require("client", "service", "1.0.0", "/p", "GET", false, "openapi")
	if err == nil || err.Error() != expected {
		t.Errorf("expected memoized wrong-server diagnosis, got %v", err)
	}
	if versionCalls != 1 {
		t.Errorf("expected /version to be called once in total, got %d", versionCalls)
	}
}

func TestSanshainClient_NoDiagnosisWhenServerIs2x(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			serveVersion(w, "2.1.0")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"info.version is not strict semver"}`))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	_, err := client.Provide(ProvidePayload{ProducerName: "test", OpenApiYaml: "x", Stability: "snapshot"}, false)
	if err == nil {
		t.Fatal("expected 400 error")
	}
	if strings.Contains(err.Error(), "upgrade the server") {
		t.Errorf("2.x server must keep the original error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "status 400") {
		t.Errorf("expected original 400 error, got %q", err.Error())
	}
}
