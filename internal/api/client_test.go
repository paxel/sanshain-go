package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSanshainClient_Provide(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/provide" {
			t.Errorf("expected /provide, got %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"version":1,"content_hash":"sha256:abc","changes":{"inserts":1,"updates":0,"deletes":0}}`))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	payload := ProvidePayload{
		ServiceName: "test",
		Branch:      "main",
		OpenApiYaml: "test",
	}
	resp, err := client.Provide(payload, false)
	if err != nil {
		t.Errorf("Provide failed: %v", err)
	}
	if resp == nil {
		t.Error("expected non-nil response")
	} else if resp.Version != 1 {
		t.Errorf("expected version 1, got %d", resp.Version)
	}
}

func TestSanshainClient_Require(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/require" {
			t.Errorf("expected /require, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("clientname") != "client" {
			t.Errorf("expected client, got %s", r.URL.Query().Get("clientname"))
		}
		_, _ = w.Write([]byte("openapi-content"))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	res, err := client.Require("client", "service", "main", "/path", "GET", 0, false, "openapi")
	if err != nil {
		t.Errorf("Require failed: %v", err)
	}
	if res != "openapi-content" {
		t.Errorf("expected openapi-content, got %s", res)
	}
}

func TestSanshainClient_RequireBundle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/require-bundle" {
			t.Errorf("expected /require-bundle, got %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("bundle-content"))
	}))
	defer ts.Close()

	client := NewSanshainClient(ts.URL, "token", false)
	payload := RequireBundlePayload{
		ClientName:  "client",
		ServiceName: "service",
		Branch:      "main",
		Endpoints:   []RequireBundleEndpoint{{Path: "/path", Method: "GET"}},
	}
	res, err := client.RequireBundle(payload, false)
	if err != nil {
		t.Errorf("RequireBundle failed: %v", err)
	}
	if res != "bundle-content" {
		t.Errorf("expected bundle-content, got %s", res)
	}
}
