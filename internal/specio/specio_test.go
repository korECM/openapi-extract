package specio

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLoadInvalidInputProducesFriendlyError(t *testing.T) {
	_, err := Load("-", strings.NewReader("random non-spec text"))
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not a valid OpenAPI document") {
		t.Fatalf("error should be friendly: %s", msg)
	}
	if strings.Contains(msg, "openapi3.") {
		t.Fatalf("error should not leak kin-openapi internal types: %s", msg)
	}
}

func TestLoadFromURL(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /health:
    get:
      summary: Health
      responses:
        "200":
          description: ok
`
	oldClient := httpClient
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(source)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	t.Cleanup(func() {
		httpClient = oldClient
	})

	loaded, err := Load("https://example.com/openapi.yaml", strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Doc.OpenAPI != "3.0.3" {
		t.Fatalf("OpenAPI version = %q", loaded.Doc.OpenAPI)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
