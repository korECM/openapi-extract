package extractor

import (
	"strings"
	"testing"

	"github.com/korECM/openapi-extract/internal/catalog"
	"github.com/korECM/openapi-extract/internal/specio"
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

	mini, err := Extract(loaded.Raw, selected, Options{})
	if err != nil {
		t.Fatal(err)
	}
	paths := mustOrderedMap(t, mini, "paths")
	if paths.Len() != 1 {
		t.Fatalf("len(paths) = %d, want 1", paths.Len())
	}
	playerPath := mustOrderedMap(t, paths, "/players/{player_id}")
	if _, ok := playerPath.Get("get"); !ok {
		t.Fatal("selected get operation missing")
	}
	if _, ok := playerPath.Get("delete"); ok {
		t.Fatal("unselected delete operation included")
	}

	components := mustOrderedMap(t, mini, "components")
	schemas := mustOrderedMap(t, components, "schemas")
	for _, name := range []string{"Player", "Profile", "Error"} {
		if _, ok := schemas.Get(name); !ok {
			t.Fatalf("reachable schema %s missing", name)
		}
	}
	if _, ok := schemas.Get("Unused"); ok {
		t.Fatal("unused schema included")
	}
	parameters := mustOrderedMap(t, components, "parameters")
	if _, ok := parameters.Get("PlayerID"); !ok {
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

	mini, err := Extract(loaded.Raw, selected, Options{})
	if err != nil {
		t.Fatal(err)
	}
	components := mustOrderedMap(t, mini, "components")
	schemes := mustOrderedMap(t, components, "securitySchemes")
	if _, ok := schemes.Get("bearerAuth"); !ok {
		t.Fatal("referenced security scheme missing")
	}
	if _, ok := schemes.Get("unusedKey"); ok {
		t.Fatal("unused security scheme included")
	}
}

func TestExtractStripsInfoDescriptionWhenRequested(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
  description: |
    Long preamble about auth, rate limits, error codes.
    Spans many lines and bloats every extracted mini spec.
paths:
  /health:
    get:
      responses:
        "200":
          description: ok
`
	loaded, err := specio.Load("-", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ops := catalog.Build(loaded.Doc)
	selected, err := catalog.Find(ops, []string{"get_/health"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	stripped, err := Extract(loaded.Raw, selected, Options{StripInfoDescription: true})
	if err != nil {
		t.Fatal(err)
	}
	info := mustOrderedMap(t, stripped, "info")
	if _, ok := info.Get("description"); ok {
		t.Fatal("info.description should be stripped")
	}
	if _, ok := info.Get("title"); !ok {
		t.Fatal("info.title should be preserved")
	}

	kept, err := Extract(loaded.Raw, selected, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rawInfo, ok := kept.Get("info")
	if !ok {
		t.Fatal("info missing in default extract")
	}
	infoMap, ok := rawInfo.(map[string]any)
	if !ok {
		t.Fatalf("default info is %T, want map[string]any", rawInfo)
	}
	if _, ok := infoMap["description"]; !ok {
		t.Fatal("info.description should be preserved by default")
	}
}

type orderedGetter interface {
	Get(string) (any, bool)
	Len() int
}

func mustOrderedMap(t *testing.T, parent orderedGetter, key string) orderedGetter {
	t.Helper()
	value, ok := parent.Get(key)
	if !ok {
		t.Fatalf("missing key %s", key)
	}
	child, ok := value.(orderedGetter)
	if !ok {
		t.Fatalf("key %s is %T, want ordered map", key, value)
	}
	return child
}
