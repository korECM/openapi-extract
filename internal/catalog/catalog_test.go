package catalog

import (
	"strings"
	"testing"

	"github.com/korECM/openapi-extract/internal/specio"
)

func TestBuildCatalog(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /players/{player_id}:
    get:
      operationId: getPlayer
      summary: Get a player
      tags: [Players]
      responses:
        "200":
          description: ok
`
	loaded, err := specio.Load("-", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}

	ops := Build(loaded.Doc)
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	op := ops[0]
	if op.ID != "get_/players/{player_id}" {
		t.Fatalf("op.ID = %q", op.ID)
	}
	if op.Method != "GET" || op.Path != "/players/{player_id}" {
		t.Fatalf("operation target = %s %s", op.Method, op.Path)
	}
	if op.OperationID != "getPlayer" || op.Summary != "Get a player" {
		t.Fatalf("metadata = %#v", op)
	}
}

func TestBuildIncludesDescriptionDeprecatedAndSecurity(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
security:
  - bearerAuth: []
paths:
  /old:
    get:
      operationId: oldOp
      description: |
        First useful line.

        Rest of long description.
      deprecated: true
      responses:
        "200":
          description: ok
  /me:
    get:
      operationId: getMe
      responses:
        "200":
          description: ok
  /open:
    get:
      security: []
      responses:
        "200":
          description: ok
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
`
	loaded, err := specio.Load("-", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ops := Build(loaded.Doc)
	byID := map[string]Operation{}
	for _, op := range ops {
		byID[op.ID] = op
	}

	deprecated := byID["get_/old"]
	if !deprecated.Deprecated {
		t.Fatal("get_/old should be marked deprecated")
	}
	if deprecated.Description != "First useful line." {
		t.Fatalf("description = %q, want first useful line", deprecated.Description)
	}

	me := byID["get_/me"]
	if len(me.Security) != 1 || me.Security[0] != "bearerAuth" {
		t.Fatalf("expected bearerAuth security to be inherited; got %v", me.Security)
	}

	open := byID["get_/open"]
	if open.Security != nil {
		t.Fatalf("get_/open opted out of security, got %v", open.Security)
	}
}

func TestFindByIDAndSelector(t *testing.T) {
	ops := []Operation{
		{ID: "get_/health", Method: "GET", Path: "/health"},
		{ID: "post_/players", Method: "POST", Path: "/players"},
	}

	selected, err := Find(ops, []string{"get_/health"}, []string{"POST /players"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.Operations) != 2 {
		t.Fatalf("len(selected.Operations) = %d, want 2", len(selected.Operations))
	}
	if len(selected.Missing) != 0 {
		t.Fatalf("unexpected misses: %v", selected.Missing)
	}
}

func TestFindReportsPartialMissesInsteadOfAborting(t *testing.T) {
	ops := []Operation{
		{ID: "get_/health", Method: "GET", Path: "/health"},
		{ID: "post_/players", Method: "POST", Path: "/players"},
	}

	res, err := Find(ops, []string{"get_/health", "get_/nope"}, []string{"DELETE /players"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Operations) != 1 || res.Operations[0].ID != "get_/health" {
		t.Fatalf("got operations %#v", res.Operations)
	}
	if len(res.Missing) != 2 {
		t.Fatalf("missing = %v, want 2 entries", res.Missing)
	}
}

func TestFindAcceptsCaseInsensitiveMethodInID(t *testing.T) {
	ops := []Operation{
		{ID: "post_/v1/orders", Method: "POST", Path: "/v1/orders"},
	}
	res, err := Find(ops, []string{"POST_/v1/orders"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Operations) != 1 || res.Operations[0].ID != "post_/v1/orders" {
		t.Fatalf("got %#v", res.Operations)
	}
}

func TestFindKeepsPathCaseSensitive(t *testing.T) {
	ops := []Operation{
		{ID: "post_/v1/orders", Method: "POST", Path: "/v1/orders"},
	}
	res, err := Find(ops, []string{"post_/V1/orders"}, nil)
	if err == nil {
		t.Fatalf("expected miss for path with different case; got %#v", res.Operations)
	}
}

func TestFindReturnsErrorWhenNothingMatches(t *testing.T) {
	ops := []Operation{{ID: "get_/health", Method: "GET", Path: "/health"}}
	if _, err := Find(ops, []string{"get_/missing"}, nil); err == nil {
		t.Fatal("expected error when no operations match")
	}
}
