# openapi-extract

[한국어 README](README.ko.md)

<p align="center">
  <img src="./introduction/out/openapi-extract-intro.gif" alt="openapi-extract 30-second introduction" width="100%">
</p>

`openapi-extract` turns a large OpenAPI 3.x document into a small, AI-friendly mini spec for only the endpoints you care about.

Stop pasting a whole API contract into an LLM when you only need `GET /players/{id}`. `openapi-extract` lists operations, lets you pick the relevant ones, and outputs a valid mini OpenAPI spec with just the selected paths plus the `$ref` components they need.

Why it helps:

- Massive prompt savings: extracting all `Orders` + `Payments` operations from a real-world spec shrinks the bundle by **~95%** versus shipping the full document. A single operation alone is typically ~66% smaller.
- Better agent focus: agents can ask for an operation catalog first, then extract by stable operation id (or tag) instead of reading the whole spec.
- Safer mini specs: selected operations keep reachable schemas, responses, parameters, headers, request bodies, and security schemes.
- Human-friendly too: use the TUI to search, multi-select, copy, or save without hand-editing YAML.

Built for two workflows:

- Humans open a terminal UI, search operations, select endpoints, then copy or save the extracted spec.
- AI agents and scripts list operation IDs first, then extract only what they need.

## Install / Build

Install the latest tagged release (needs **v0.3.0+** for `--tag`, `--method`,
`--exclude-tag`, `--verbose`, `--quiet`, JSON warnings/summary, and the
extended JSON catalog fields):

```bash
go install github.com/korECM/openapi-extract@latest
```

Install the latest commit on `main` (useful when new flags landed since the
last tag):

```bash
go install github.com/korECM/openapi-extract@main
```

Confirm which version is on `$PATH`:

```bash
openapi-extract --version
```

Make sure your Go bin directory is on `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Build from a local checkout:

```bash
go build ./...
go build -o openapi-extract .
```

## Interactive TUI

```bash
openapi-extract ./openapi.yaml
openapi-extract https://docs.example.com/openapi.yaml
cat openapi.yaml | openapi-extract -
```

TUI keys:

- `j/k` or arrow keys: move
- `/`: search operations by method, path, tag, summary, or operationId
- `space`: select or unselect an operation
- `c`: copy the selected mini spec to the clipboard
- `s`: save the selected mini spec to a file
- `y`: print the selected mini spec to stdout and quit
- `q` or `esc`: quit

## AI Agent Workflow

Use `list` first. The agent gets a compact operation catalog and does not need
to inspect the full OpenAPI document. The JSON view carries enough metadata
(`hasRequestBody`, `requestBodyRequired`, `responseCodes`, `deprecated`,
`security`) to decide what to extract without opening the source spec.

```bash
openapi-extract list /Users/devsisters/Downloads/scalar-galaxy.yaml --format json
openapi-extract list https://docs.example.com/openapi.yaml --format json
```

**The killer move is `--tag` extraction.** Pull every operation under a
domain in a single call:

```bash
openapi-extract extract api.yaml --tag Orders --tag Payments --stdout
# stderr: extracted 27 ops / 41 schemas: 891,204 → 43,118 bytes (95% smaller)
```

Single-operation extraction is still there when you want it:

```bash
openapi-extract extract api.yaml --id 'get_/v1/orders/{id}' --stdout
```

Combine tag-pull with post-selection filters for surgical slices:

```bash
openapi-extract extract api.yaml --tag Orders --method GET --no-deprecated --stdout
openapi-extract extract api.yaml --tag Orders --tag Payments --exclude-tag Webhooks --stdout
```

Multiple ids, human-readable selectors, and the `--copy` / `--output` sinks
all work the same way as before:

```bash
openapi-extract extract api.yaml --id 'get_/v1/orders' --id 'post_/v1/orders' --stdout
openapi-extract extract api.yaml --select 'GET /v1/orders/{id}' --output mini.yaml
```

On success, a one-line size summary is written to **stderr** so it does not
pollute piped output:

```text
extracted 3 ops / 14 schemas: 44,250 → 14,837 bytes (66% smaller)
```

Use `--quiet` to suppress it, or `--format json` to receive the summary (and
any `missing_id` warnings) as JSONL on stderr.

## Agent Integrations

This repository includes reusable instructions and plugin metadata for multiple coding agents.

- Codex: `plugins/openapi-extract/.codex-plugin/plugin.json`, `.agents/plugins/marketplace.json`
- Claude Code: `plugins/openapi-extract/.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`
- Cursor: `.cursor/rules/openapi-extract.mdc`
- OpenCode and generic agents: `AGENTS.md`
- Shared skill: `skills/openapi-extract/SKILL.md`

Quick installs:

```bash
# Claude Code plugin marketplace
/plugin marketplace add korECM/openapi-extract
/plugin install openapi-extract@openapi-extract-marketplace

# Agent Skills CLI
npx skills add korECM/openapi-extract --skill openapi-extract
```

See [docs/agent-integrations.md](docs/agent-integrations.md) for install and usage details.

## CLI Reference

List operations:

```bash
openapi-extract list <openapi.yaml|url|-> \
  [--format text|tree|json] \
  [--columns id,method,path,operationId,summary,description,tags,deprecated,security,body,responseCodes] \
  [--tag NAME] [--exclude-tag NAME] \
  [--method NAMES] [--path-prefix /v1] [--grep PATTERN] \
  [--no-deprecated] [--max-col-width N] [--no-header] [--no-color] \
  [--no-cache] [--refresh-cache] [--verbose]
```

Filters combine with AND across types and OR within each repeatable type:

```bash
openapi-extract list ./openapi.yaml --method GET,POST --path-prefix /v1/orders
openapi-extract list ./openapi.yaml --grep refund --no-deprecated
```

Text output supports selectable columns. The JSON catalog object includes:

```text
id, method, path, operationId, summary, description, tags,
deprecated, security, hasRequestBody, requestBodyRequired,
responseCodes, kind
```

so the catalog stage alone can answer "any deprecated ops?", "which need
auth?", "which require a request body?", or "which return 4xx codes?".

`--format tree` groups operations by path, which is easier to skim when the
same path serves many methods.

```bash
openapi-extract list ./openapi.yaml --columns method,path,tags,operationId
openapi-extract list ./openapi.yaml --columns all
openapi-extract list ./openapi.yaml --tag Orders --tag Payments
openapi-extract list https://docs.example.com/openapi.yaml --max-col-width 60
```

`openapi-extract list --help` and `openapi-extract extract --help` print full
flag references and exit with code 0.

Extract selected operations:

```bash
openapi-extract extract <openapi.yaml|url|-> \
  (--id ID | --select 'METHOD /path' | --tag NAME) \
  [--exclude-tag NAME] [--method NAMES] [--path-prefix /v1] \
  [--grep PATTERN] [--no-deprecated] \
  (--stdout | --copy | --output mini.openapi.yaml) \
  [--format yaml|json] \
  [--strip-info-description | --keep-info-description] \
  [--max-enum N] [--drop-examples] \
  [--no-cache] [--refresh-cache] [--quiet] [--verbose]
```

Notes:

- `--id` is case-insensitive on the method portion (`POST_/v1/orders` and
  `post_/v1/orders` resolve the same operation). Paths stay case-sensitive.
- Missing ids no longer abort the call. Each miss is printed to stderr as a
  warning and the successful subset is still extracted; the command only
  fails when zero operations matched.
- `--tag Orders` pulls every operation under that tag into the mini spec.
- Post-selection filters (`--exclude-tag`, `--method`, `--path-prefix`,
  `--grep`, `--no-deprecated`) narrow the chosen set with AND semantics. They
  match the filters on `list`, so an agent can stage a selection with `--tag`
  and refine it in the same call without piping through `xargs`.
- On success, a one-line size summary is written to stderr unless `--quiet`
  is set: `extracted 3 ops / 14 schemas: 44,250 → 14,837 bytes (66% smaller)`.
- With `--format json`, both `missing_id` warnings and the size summary are
  emitted as JSONL on stderr instead of plain text.
- OpenAPI 3.1 top-level `webhooks` (e.g. `player.verify`) appear in the
  catalog with ids of the form `webhook_<name>_<method>` and extract into a
  matching `webhooks:` block.
- `info.description` is stripped from every mini spec by default so the
  marketing/auth/rate-limit preamble does not bloat AI prompts, regardless
  of `--stdout`, `--copy`, or `--output`. Use `--keep-info-description` to
  preserve it; `--strip-info-description` is the (default) explicit form.
- `--max-enum N` truncates JSON Schema `enum` arrays longer than N to
  their first N entries and writes a sibling `x-enum-truncated: {kept,
  total}` marker. Useful for ISO-4217/country/locale enums that bloat
  prompts.
- `--drop-examples` removes `example` and `examples` keys at every depth
  of the mini spec. Useful when shipping the spec to LLMs that do not
  need sample payloads.
- URL fetches are cached under `$OPENAPI_EXTRACT_CACHE_DIR` (default
  `os.UserCacheDir()/openapi-extract`) and revalidated with `ETag` /
  `Last-Modified`. `--no-cache` skips the cache; `--refresh-cache`
  overwrites it.

Input sources:

- Local file: `openapi-extract list ./openapi.yaml`
- URL: `openapi-extract list https://docs.example.com/openapi.yaml --format json`
- stdin: `cat openapi.yaml | openapi-extract list - --format json`

Output targets:

- `--stdout`: print the mini spec
- `--copy`: copy the mini spec to the clipboard
- `--output <path>`: save the mini spec to a file

## What Gets Extracted

The mini spec keeps only the selected operations and the OpenAPI pieces needed to make those operations usable:

- selected `paths` and HTTP methods
- path-level parameters for selected paths
- recursively reachable `$ref` components
- referenced `securitySchemes`
- root `openapi`, `info`, `servers`, `security`, `tags`, and `externalDocs` where applicable

The extractor removes unrelated paths and unused components, which keeps the output smaller for LLM prompts.
