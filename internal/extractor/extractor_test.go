package extractor

import (
	"strings"
	"testing"

	"github.com/devsisters/openapi-extract/internal/catalog"
	"github.com/devsisters/openapi-extract/internal/specio"
)

func TestExtractIncludesOnlySelectedOperationAndReachableRefs(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /players/{player_id}:
    parameters:
      - $ref: "#/components/parameters/PlayerID"
    get:
      operationId: getPlayer
      tags: [Players]
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Player"
        "404":
          $ref: "#/components/responses/NotFound"
    delete:
      responses:
        "204":
          description: deleted
  /teams:
    get:
      responses:
        "200":
          description: ok
components:
  parameters:
    PlayerID:
      name: player_id
      in: path
      required: true
      schema:
        type: string
  responses:
    NotFound:
      description: missing
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/Error"
  schemas:
    Player:
      type: object
      properties:
        profile:
          $ref: "#/components/schemas/Profile"
    Profile:
      type: object
      properties:
        name:
          type: string
    Error:
      type: object
      properties:
        message:
          type: string
    Unused:
      type: object
`
	loaded, err := specio.Load("-", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ops := catalog.Build(loaded.Doc)
	selected, err := catalog.Find(ops, []string{"get_/players/{player_id}"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	mini, err := Extract(loaded.Raw, selected)
	if err != nil {
		t.Fatal(err)
	}
	paths := mini["paths"].(map[string]any)
	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1", len(paths))
	}
	playerPath := paths["/players/{player_id}"].(map[string]any)
	if _, ok := playerPath["get"]; !ok {
		t.Fatal("selected get operation missing")
	}
	if _, ok := playerPath["delete"]; ok {
		t.Fatal("unselected delete operation included")
	}

	components := mini["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	for _, name := range []string{"Player", "Profile", "Error"} {
		if _, ok := schemas[name]; !ok {
			t.Fatalf("reachable schema %s missing", name)
		}
	}
	if _, ok := schemas["Unused"]; ok {
		t.Fatal("unused schema included")
	}
	parameters := components["parameters"].(map[string]any)
	if _, ok := parameters["PlayerID"]; !ok {
		t.Fatal("path-level parameter ref missing")
	}
}

func TestExtractIncludesReferencedSecuritySchemes(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
security:
  - bearerAuth: []
paths:
  /me:
    get:
      responses:
        "200":
          description: ok
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
    unusedKey:
      type: apiKey
      in: header
      name: X-API-Key
`
	loaded, err := specio.Load("-", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ops := catalog.Build(loaded.Doc)
	selected, err := catalog.Find(ops, []string{"get_/me"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	mini, err := Extract(loaded.Raw, selected)
	if err != nil {
		t.Fatal(err)
	}
	components := mini["components"].(map[string]any)
	schemes := components["securitySchemes"].(map[string]any)
	if _, ok := schemes["bearerAuth"]; !ok {
		t.Fatal("referenced security scheme missing")
	}
	if _, ok := schemes["unusedKey"]; ok {
		t.Fatal("unused security scheme included")
	}
}
