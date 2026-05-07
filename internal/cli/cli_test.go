package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestListJSONAndExtractStdout(t *testing.T) {
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
	var listOut bytes.Buffer
	if code := Run([]string{"list", "-", "--format", "json"}, strings.NewReader(source), &listOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("list exit code = %d", code)
	}
	if !strings.Contains(listOut.String(), `"id": "get_/health"`) {
		t.Fatalf("list output missing operation id: %s", listOut.String())
	}

	var extractOut bytes.Buffer
	if code := Run([]string{"extract", "-", "--id", "get_/health", "--stdout"}, strings.NewReader(source), &extractOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("extract exit code = %d", code)
	}
	out := extractOut.String()
	if !strings.Contains(out, "/health:") || strings.Contains(out, "/missing:") {
		t.Fatalf("unexpected extract output: %s", out)
	}
}
