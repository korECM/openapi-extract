# openapi-extract

[한국어 README](README.ko.md)

<p align="center">
  <video src="./introduction/out/openapi-extract-intro-1080p.mp4" controls width="100%"></video>
</p>

`openapi-extract` turns a large OpenAPI 3.x document into a small, AI-friendly mini spec for only the endpoints you care about.

Stop pasting a whole API contract into an LLM when you only need `GET /players/{id}`. `openapi-extract` lists operations, lets you pick the relevant ones, and outputs a valid mini OpenAPI spec with just the selected paths plus the `$ref` components they need.

Why it helps:

- Smaller prompts: in the Scalar Galaxy sample, one operation shrinks from 44,250 bytes / 1,450 lines to 14,837 bytes / 514 lines, about 66% smaller by bytes.
- Better agent focus: agents can ask for an operation catalog first, then extract by stable operation id instead of reading the whole spec.
- Safer mini specs: selected operations keep reachable schemas, responses, parameters, headers, request bodies, and security schemes.
- Human-friendly too: use the TUI to search, multi-select, copy, or save without hand-editing YAML.

Built for two workflows:

- Humans open a terminal UI, search operations, select endpoints, then copy or save the extracted spec.
- AI agents and scripts list operation IDs first, then extract only what they need.

## Install / Build

Install the latest released commit:

```bash
go install github.com/korECM/openapi-extract@latest
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

Use `list` first. The agent gets a compact operation catalog and does not need to inspect the full OpenAPI document.

```bash
openapi-extract list /Users/devsisters/Downloads/scalar-galaxy.yaml --format json
openapi-extract list https://docs.example.com/openapi.yaml --format json
```

Example text catalog:

```text
post_/auth/token              POST /auth/token - Get a token
get_/me                       GET /me - Get authenticated user
get_/planets                  GET /planets - Get all planets
get_/planets/{planetId}       GET /planets/{planetId} - Get a planet
```

Then extract by `id`:

```bash
openapi-extract extract /Users/devsisters/Downloads/scalar-galaxy.yaml \
  --id 'get_/planets/{planetId}' \
  --stdout
```

Multiple operations are supported:

```bash
openapi-extract extract /Users/devsisters/Downloads/scalar-galaxy.yaml \
  --id 'get_/planets' \
  --id 'get_/planets/{planetId}' \
  --stdout
```

Human-readable selectors are also supported:

```bash
openapi-extract extract /Users/devsisters/Downloads/scalar-galaxy.yaml \
  --select 'GET /planets/{planetId}' \
  --stdout
```

The output remains a real OpenAPI document, just smaller:

```text
Scalar Galaxy full spec:              44,250 bytes / 1,450 lines
GET /planets/{planetId} mini spec:    14,837 bytes /   514 lines
Reduction:                            about 66% by bytes
```

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
openapi-extract list <openapi.yaml|url|-> [--format text|json] [--columns id,method,path,summary] [--no-header] [--no-color]
```

Text output supports selectable columns:

```bash
openapi-extract list ./openapi.yaml --columns method,path,tags,operationId
openapi-extract list ./openapi.yaml --columns all
```

Extract selected operations:

```bash
openapi-extract extract <openapi.yaml|url|-> \
  (--id ID | --select 'METHOD /path') \
  (--stdout | --copy | --output mini.openapi.yaml) \
  [--format yaml|json]
```

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
