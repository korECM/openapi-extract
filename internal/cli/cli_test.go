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

func TestListTextIsAlignedWithoutColorForNonTTY(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /a:
    get:
      summary: A
      responses:
        "200":
          description: ok
  /long/path:
    post:
      summary: Long
      responses:
        "200":
          description: ok
`
	var listOut bytes.Buffer
	if code := Run([]string{"list", "-"}, strings.NewReader(source), &listOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("list exit code = %d", code)
	}
	out := listOut.String()
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("non-TTY output should not contain ANSI escapes: %q", out)
	}
	if !strings.Contains(out, "ID               METHOD  PATH        SUMMARY") {
		t.Fatalf("header row not aligned as expected: %q", out)
	}
	if !strings.Contains(out, "get_/a           GET     /a          A") {
		t.Fatalf("GET row not aligned as expected: %q", out)
	}
	if !strings.Contains(out, "post_/long/path  POST    /long/path  Long") {
		t.Fatalf("POST row not aligned as expected: %q", out)
	}
}

func TestListTextColumns(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /health:
    get:
      operationId: getHealth
      summary: Health
      tags: [System]
      responses:
        "200":
          description: ok
`
	var listOut bytes.Buffer
	args := []string{"list", "-", "--columns", "method,path,tags,operationId", "--no-header"}
	if code := Run(args, strings.NewReader(source), &listOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("list exit code = %d", code)
	}
	out := listOut.String()
	if strings.Contains(out, "ID") || strings.Contains(out, "SUMMARY") {
		t.Fatalf("unexpected default columns/header: %q", out)
	}
	if !strings.Contains(out, "GET     /health  System  getHealth") {
		t.Fatalf("custom columns not rendered as expected: %q", out)
	}
}
