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

func TestFindByIDAndSelector(t *testing.T) {
	ops := []Operation{
		{ID: "get_/health", Method: "GET", Path: "/health"},
		{ID: "post_/players", Method: "POST", Path: "/players"},
	}

	selected, err := Find(ops, []string{"get_/health"}, []string{"POST /players"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 {
		t.Fatalf("len(selected) = %d, want 2", len(selected))
	}
}
