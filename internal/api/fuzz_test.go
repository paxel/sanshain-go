package api

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestWarningLines_OneLinePerHeaderLineInOrder(t *testing.T) {
	header := http.Header{}
	header.Add("Sanshain-Warning", "first, with a comma")
	header.Add("Other", "ignored")
	header.Add("sanshain-warning", "second")
	got := warningLines(header)
	if len(got) != 2 || got[0] != "first, with a comma" || got[1] != "second" {
		t.Errorf("unexpected warnings %q", got)
	}
	if lines := warningLines(http.Header{}); len(lines) != 0 {
		t.Errorf("expected no warnings, got %q", lines)
	}
}

func TestDecodeProvideResponse(t *testing.T) {
	resp, err := decodeProvideResponse(`{"version":"1.2.0","stability":"ga","content_hash":"sha256:a","changes":{"inserts":1,"updates":2,"deletes":3}}`)
	if err != nil {
		t.Fatalf("expected the answer to be read, got %v", err)
	}
	if resp.Version != "1.2.0" || resp.Stability != "ga" || resp.Changes != (ProvideChanges{Inserts: 1, Updates: 2, Deletes: 3}) {
		t.Errorf("unexpected answer %+v", resp)
	}
	if _, err := decodeProvideResponse("<html>"); err == nil {
		t.Error("expected an error for a body that is not JSON")
	}
}

// FuzzAnnotations fills the placeholders of annotation values: any value, any
// --set pair and any environment, never a panic, and every entry is either
// kept or left out with a warning.
//
// The first line holds the annotation values, separated by tabs. Every further
// line is a NAME=value pair, by turns a --set pair and an environment variable.
func FuzzAnnotations(f *testing.F) {
	f.Add("build {{ID}}\t{{TEAM}}\nID=42\nTEAM=core")
	f.Add("{{MISSING}}\tplain\t{{ a }}{{}}\nMISSING=")
	f.Add("{{A}}{{B}}{{A}}\nA={{B}}\nB=x\nA=y")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		lines := strings.Split(text, "\n")
		var declared []Annotation
		for i, value := range strings.Split(lines[0], "\t") {
			scope := "version"
			if i%2 == 1 {
				scope = "system"
			}
			declared = append(declared, Annotation{Scope: scope, Name: "annotation", Value: value})
		}
		sets := map[string]string{}
		environment := map[string]string{}
		for i, line := range lines[1:] {
			name, value, found := strings.Cut(line, "=")
			if !found {
				continue
			}
			if i%2 == 0 {
				sets[name] = value
			} else {
				environment[SetEnvPrefix+name] = value
			}
		}
		filled, warnings := FillAnnotations(declared, sets, func(name string) string { return environment[name] })
		if len(filled)+len(warnings) != len(declared) {
			t.Fatalf("%d declared, but %d kept and %d left out", len(declared), len(filled), len(warnings))
		}
	})
}

// FuzzResponse reads what the service answers: any body, any version and any
// Sanshain-Warning header values, never a panic.
//
// The input is read as the body of an answer, failed or successful, as the
// version the service names, and, split at line feeds, as the values of the
// warning headers.
func FuzzResponse(f *testing.F) {
	f.Add([]byte(`{"error":"version 1.2.0 is GA and immutable","proposed_version":"1.2.1"}`))
	f.Add([]byte(`{"version":"1.2.0","stability":"snapshot","content_hash":"sha256:a","changes":{"inserts":1,"updates":0,"deletes":0},"harvested_subscriptions":[{"channel":"c","message_name":"m","owner":"","drift":"d"}]}`))
	f.Add([]byte("unknown field `annotations`, expected one of `producername`, `build_url`\nsecond line"))
	f.Add([]byte("<html>\x00\xff bad gateway</html>"))
	f.Add([]byte(`{"error":7,"changes":[]}`))
	f.Add([]byte("openapi: 3.0.3\ninfo:\n  version: 1.2.0\n"))
	f.Add([]byte("v2.13.0-rc1"))
	f.Add([]byte("1.9"))
	f.Add([]byte(""))
	f.Fuzz(func(t *testing.T, data []byte) {
		body := string(data)
		if eb := parseErrorBody(body); eb.Error == "" && body != "" {
			t.Fatalf("expected a message for the body %q", body)
		}
		if resp, err := decodeProvideResponse(body); (resp == nil) == (err == nil) {
			t.Fatalf("expected an answer or an error, got %v and %v", resp, err)
		}
		_ = annotationsFieldRefused(body)
		_ = originFieldRefused(body)
		// A require that asked for a format looks into the document it got.
		_ = isOpenAPI(body)
		if major, ok := parseMajor(body); !ok && major != 0 {
			t.Fatalf("expected no major version for %q, got %d", body, major)
		}
		if atLeast(body, 2, 4, 0) && !atLeast(body, 2, 0, 0) {
			t.Fatalf("%q is at least 2.4.0 but not at least 2.0.0", body)
		}

		values := bytes.Split(data, []byte("\n"))
		header := http.Header{}
		for _, value := range values {
			header.Add("Sanshain-Warning", string(value))
		}
		if lines := warningLines(header); len(lines) != len(values) {
			t.Fatalf("%d header lines, but %d warnings", len(values), len(lines))
		}
	})
}
