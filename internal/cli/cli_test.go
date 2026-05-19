package cli

import (
	"bytes"
	"os"
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

func TestListFiltersByTag(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /orders:
    get:
      tags: [Orders]
      summary: List orders
      responses:
        "200":
          description: ok
  /players:
    get:
      tags: [Players]
      summary: List players
      responses:
        "200":
          description: ok
`
	var out bytes.Buffer
	if code := Run([]string{"list", "-", "--format", "json", "--tag", "orders"}, strings.NewReader(source), &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("list exit code = %d", code)
	}
	body := out.String()
	if !strings.Contains(body, `"id": "get_/orders"`) {
		t.Fatalf("Orders op missing: %s", body)
	}
	if strings.Contains(body, `"id": "get_/players"`) {
		t.Fatalf("Players op should be filtered out: %s", body)
	}
}

func TestExtractSelectsByTag(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /orders:
    get:
      tags: [Orders]
      responses:
        "200":
          description: ok
  /players:
    get:
      tags: [Players]
      responses:
        "200":
          description: ok
`
	var out bytes.Buffer
	if code := Run([]string{"extract", "-", "--tag", "Orders", "--stdout"}, strings.NewReader(source), &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("extract exit code = %d", code)
	}
	body := out.String()
	if !strings.Contains(body, "/orders:") {
		t.Fatalf("Orders path missing: %s", body)
	}
	if strings.Contains(body, "/players:") {
		t.Fatalf("Players path should not be included: %s", body)
	}
}

func TestExtractWarnsOnPartialMissAndStillSucceeds(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /health:
    get:
      responses:
        "200":
          description: ok
`
	var out, errBuf bytes.Buffer
	args := []string{"extract", "-", "--id", "get_/health", "--id", "get_/nope", "--stdout"}
	if code := Run(args, strings.NewReader(source), &out, &errBuf); code != 0 {
		t.Fatalf("exit code = %d (stderr=%s)", code, errBuf.String())
	}
	if !strings.Contains(out.String(), "/health:") {
		t.Fatalf("successful op missing from output: %s", out.String())
	}
	if !strings.Contains(errBuf.String(), "warning: operation not found: get_/nope") {
		t.Fatalf("missing warning in stderr: %s", errBuf.String())
	}
}

func TestExtractFailsWhenAllMissing(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /health:
    get:
      responses:
        "200":
          description: ok
`
	var out, errBuf bytes.Buffer
	args := []string{"extract", "-", "--id", "get_/nope", "--stdout"}
	if code := Run(args, strings.NewReader(source), &out, &errBuf); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "no operations matched") {
		t.Fatalf("expected aggregate error in stderr: %s", errBuf.String())
	}
}

func TestExtractStdoutStripsInfoDescriptionByDefault(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
  description: Long preamble describing auth and rate limits.
paths:
  /health:
    get:
      responses:
        "200":
          description: ok
`
	var out bytes.Buffer
	if code := Run([]string{"extract", "-", "--id", "get_/health", "--stdout"}, strings.NewReader(source), &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("extract exit code = %d", code)
	}
	body := out.String()
	if strings.Contains(body, "Long preamble") {
		t.Fatalf("info.description should be stripped from --stdout output by default: %s", body)
	}
	if !strings.Contains(body, "title: Test API") {
		t.Fatalf("info.title should remain: %s", body)
	}

	out.Reset()
	if code := Run([]string{"extract", "-", "--id", "get_/health", "--stdout", "--keep-info-description"}, strings.NewReader(source), &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("extract --keep-info-description exit code = %d", code)
	}
	if !strings.Contains(out.String(), "Long preamble") {
		t.Fatalf("--keep-info-description should preserve description: %s", out.String())
	}
}

func TestExtractFileKeepsInfoDescriptionByDefault(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
  description: Long preamble describing auth and rate limits.
paths:
  /health:
    get:
      responses:
        "200":
          description: ok
`
	tmp := t.TempDir() + "/mini.yaml"
	if code := Run([]string{"extract", "-", "--id", "get_/health", "--output", tmp}, strings.NewReader(source), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("extract --output exit code = %d", code)
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Long preamble") {
		t.Fatalf("--output should keep info.description by default: %s", string(data))
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
