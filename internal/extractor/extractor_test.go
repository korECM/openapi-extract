package extractor

import (
	"strings"
	"testing"

	"github.com/korECM/openapi-extract/internal/catalog"
	"github.com/korECM/openapi-extract/internal/ordered"
	"github.com/korECM/openapi-extract/internal/output"
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

	mini, err := Extract(loaded.Raw, selected.Operations, Options{})
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

	mini, err := Extract(loaded.Raw, selected.Operations, Options{})
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

	stripped, err := Extract(loaded.Raw, selected.Operations, Options{StripInfoDescription: true})
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

	kept, err := Extract(loaded.Raw, selected.Operations, Options{})
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

func TestExtractIncludesWebhooks(t *testing.T) {
	const source = `
openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths: {}
webhooks:
  player.verify:
    post:
      operationId: playerVerify
      requestBody:
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/VerifyPayload"
      responses:
        "200":
          description: ok
components:
  schemas:
    VerifyPayload:
      type: object
      properties:
        playerId:
          type: string
`
	loaded, err := specio.Load("-", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ops := catalog.Build(loaded.Doc)
	var webhookID string
	for _, op := range ops {
		if op.Kind == catalog.KindWebhook {
			webhookID = op.ID
			break
		}
	}
	if webhookID == "" {
		t.Fatal("webhook operation not surfaced in catalog")
	}
	selected, err := catalog.Find(ops, []string{webhookID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	mini, err := Extract(loaded.Raw, selected.Operations, Options{})
	if err != nil {
		t.Fatal(err)
	}
	webhooks := mustOrderedMap(t, mini, "webhooks")
	hook := mustOrderedMap(t, webhooks, "player.verify")
	if _, ok := hook.Get("post"); !ok {
		t.Fatal("webhook post operation missing")
	}
	components := mustOrderedMap(t, mini, "components")
	schemas := mustOrderedMap(t, components, "schemas")
	if _, ok := schemas.Get("VerifyPayload"); !ok {
		t.Fatal("webhook-referenced schema missing")
	}
}

func TestExtractTruncatesLargeEnumWithMarker(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /pay:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                currency:
                  type: string
                  enum: [USD, EUR, JPY, KRW, GBP, CNY, AUD, CAD, CHF, SEK]
      responses:
        "200":
          description: ok
`
	loaded, err := specio.Load("-", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ops := catalog.Build(loaded.Doc)
	selected, err := catalog.Find(ops, []string{"post_/pay"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	mini, err := Extract(loaded.Raw, selected.Operations, Options{MaxEnum: 3})
	if err != nil {
		t.Fatal(err)
	}
	paths := mustOrderedMap(t, mini, "paths")
	pay := mustOrderedMap(t, paths, "/pay")
	post, ok := pay.Get("post")
	if !ok {
		t.Fatal("post operation missing")
	}
	currency := dig(t, post, "requestBody", "content", "application/json", "schema", "properties", "currency")
	enumVal, ok := getField(currency, "enum")
	if !ok {
		t.Fatalf("enum missing on currency schema: %#v", currency)
	}
	enumList, ok := enumVal.([]any)
	if !ok {
		t.Fatalf("enum is %T, want []any", enumVal)
	}
	if len(enumList) != 3 {
		t.Fatalf("enum length = %d, want 3", len(enumList))
	}
	markerVal, ok := getField(currency, "x-enum-truncated")
	if !ok {
		t.Fatalf("x-enum-truncated marker missing: %#v", currency)
	}
	markerMap, ok := markerVal.(map[string]any)
	if !ok {
		t.Fatalf("marker is %T, want map[string]any", markerVal)
	}
	if kept, _ := markerMap["kept"].(int); kept != 3 {
		t.Fatalf("marker.kept = %v, want 3", markerMap["kept"])
	}
	if total, _ := markerMap["total"].(int); total != 10 {
		t.Fatalf("marker.total = %v, want 10", markerMap["total"])
	}
}

func TestExtractLeavesSmallEnumsAlone(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /pay:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                method:
                  type: string
                  enum: [card, cash]
      responses:
        "200":
          description: ok
`
	loaded, err := specio.Load("-", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ops := catalog.Build(loaded.Doc)
	selected, err := catalog.Find(ops, []string{"post_/pay"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	mini, err := Extract(loaded.Raw, selected.Operations, Options{MaxEnum: 5})
	if err != nil {
		t.Fatal(err)
	}
	method := dig(t, mustOrderedMap(t, mustOrderedMap(t, mini, "paths"), "/pay"),
		"post", "requestBody", "content", "application/json", "schema", "properties", "method")
	if _, ok := getField(method, "x-enum-truncated"); ok {
		t.Fatalf("small enum should not be marked truncated: %#v", method)
	}
}

func TestExtractDropsExamplesAtEveryDepth(t *testing.T) {
	const source = `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /pay:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              example: { amount: 100 }
              properties:
                amount:
                  type: integer
                  example: 100
            examples:
              one:
                value: { amount: 1 }
      responses:
        "200":
          description: ok
          content:
            application/json:
              example: { ok: true }
`
	loaded, err := specio.Load("-", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	ops := catalog.Build(loaded.Doc)
	selected, err := catalog.Find(ops, []string{"post_/pay"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	mini, err := Extract(loaded.Raw, selected.Operations, Options{DropExamples: true})
	if err != nil {
		t.Fatal(err)
	}
	body, err := yamlEncode(mini)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "example:") || strings.Contains(body, "examples:") {
		t.Fatalf("examples should be removed at every depth:\n%s", body)
	}
}

func dig(t *testing.T, node any, keys ...string) any {
	t.Helper()
	cur := node
	for _, key := range keys {
		val, ok := getField(cur, key)
		if !ok {
			t.Fatalf("missing key %s in %#v", key, cur)
		}
		cur = val
	}
	return cur
}

func getField(node any, key string) (any, bool) {
	switch typed := node.(type) {
	case *ordered.Map:
		return typed.Get(key)
	case map[string]any:
		v, ok := typed[key]
		return v, ok
	default:
		return nil, false
	}
}

func yamlEncode(value any) (string, error) {
	data, err := output.Marshal(value, "yaml")
	if err != nil {
		return "", err
	}
	return string(data), nil
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
