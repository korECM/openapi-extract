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

func TestListTruncatesCellsWithMaxColWidth(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /very/deeply/nested/resource/that/is/long:
    get:
      summary: A summary that should be cut off
      responses:
        "200":
          description: ok
`
	var out bytes.Buffer
	args := []string{"list", "-", "--max-col-width", "12", "--no-header"}
	if code := Run(args, strings.NewReader(source), &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(out.String(), "…") {
		t.Fatalf("expected ellipsis in truncated output: %q", out.String())
	}
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		for _, part := range strings.Fields(line) {
			if runeCount(part) > 12 {
				t.Fatalf("cell %q exceeds 12 runes in line %q", part, line)
			}
		}
	}
}

func runeCount(s string) int { return len([]rune(s)) }

func TestSubcommandHelpExitsZero(t *testing.T) {
	for _, sub := range []string{"list", "extract"} {
		var out, errBuf bytes.Buffer
		if code := Run([]string{sub, "--help"}, strings.NewReader(""), &out, &errBuf); code != 0 {
			t.Fatalf("%s --help exit code = %d (stderr=%s)", sub, code, errBuf.String())
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("%s --help missing Usage section: %s", sub, out.String())
		}
		if errBuf.Len() != 0 {
			t.Fatalf("%s --help leaked to stderr: %s", sub, errBuf.String())
		}
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

func TestExtractFileStripsInfoDescriptionByDefault(t *testing.T) {
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
	if strings.Contains(string(data), "Long preamble") {
		t.Fatalf("--output should strip info.description by default: %s", string(data))
	}
	if !strings.Contains(string(data), "title: Test API") {
		t.Fatalf("info.title should remain: %s", string(data))
	}

	keepPath := t.TempDir() + "/mini-keep.yaml"
	if code := Run([]string{"extract", "-", "--id", "get_/health", "--output", keepPath, "--keep-info-description"}, strings.NewReader(source), &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("extract --output --keep-info-description exit code = %d", code)
	}
	keptData, err := os.ReadFile(keepPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(keptData), "Long preamble") {
		t.Fatalf("--keep-info-description should preserve description: %s", string(keptData))
	}
}

func TestListFiltersByMethodPathPrefixGrepAndDeprecated(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /v1/orders:
    get:
      summary: List orders
      responses:
        "200":
          description: ok
    post:
      summary: Create order
      responses:
        "200":
          description: ok
  /v1/orders/{id}/refund:
    post:
      summary: Refund an order
      responses:
        "200":
          description: ok
  /v1/players:
    get:
      summary: List players
      responses:
        "200":
          description: ok
  /v1/legacy:
    get:
      summary: legacy
      deprecated: true
      responses:
        "200":
          description: ok
`
	runList := func(extra ...string) string {
		args := append([]string{"list", "-", "--format", "json"}, extra...)
		var out bytes.Buffer
		if code := Run(args, strings.NewReader(source), &out, &bytes.Buffer{}); code != 0 {
			t.Fatalf("list %v exit code = %d", extra, code)
		}
		return out.String()
	}

	body := runList("--method", "GET")
	if !strings.Contains(body, `"id": "get_/v1/orders"`) || strings.Contains(body, `"method": "POST"`) {
		t.Fatalf("--method GET did not filter: %s", body)
	}

	body = runList("--method", "get,post", "--path-prefix", "/v1/orders")
	if !strings.Contains(body, `"id": "get_/v1/orders"`) {
		t.Fatalf("expected GET /v1/orders in output: %s", body)
	}
	if !strings.Contains(body, `"id": "post_/v1/orders"`) {
		t.Fatalf("expected POST /v1/orders in output: %s", body)
	}
	if strings.Contains(body, `"id": "get_/v1/players"`) {
		t.Fatalf("path-prefix should drop /v1/players: %s", body)
	}

	body = runList("--grep", "refund")
	if !strings.Contains(body, `"id": "post_/v1/orders/{id}/refund"`) {
		t.Fatalf("--grep refund missed the refund op: %s", body)
	}
	if strings.Contains(body, `"id": "get_/v1/orders"`) {
		t.Fatalf("--grep refund should drop /v1/orders: %s", body)
	}

	body = runList("--no-deprecated")
	if strings.Contains(body, `"id": "get_/v1/legacy"`) {
		t.Fatalf("--no-deprecated should drop /v1/legacy: %s", body)
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
