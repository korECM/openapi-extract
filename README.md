# openapi-extract

[한국어 README](README.ko.md)

`openapi-extract` is a Go TUI and CLI for turning a large OpenAPI 3.x document into a small, AI-friendly mini spec.

It is built for two workflows:

- Humans can open a terminal UI, search operations, select a few endpoints, then copy or save the extracted spec.
- AI agents and scripts can list operation IDs first, then extract only the operations they need without reading the full OpenAPI file directly.

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

## Agent Integrations

This repository includes reusable instructions and plugin metadata for multiple coding agents.

- Codex: `plugins/openapi-extract/.codex-plugin/plugin.json`, `.agents/plugins/marketplace.json`
- Claude Code: `plugins/openapi-extract/.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`
- Cursor: `.cursor/rules/openapi-extract.mdc`
- OpenCode and generic agents: `AGENTS.md`
- Shared skill: `plugins/openapi-extract/skills/openapi-extract/SKILL.md`

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
