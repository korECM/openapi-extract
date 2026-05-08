---
name: openapi-extract
description: Use when an AI agent needs a small OpenAPI 3.x mini spec for selected endpoints without reading an entire OpenAPI document. Helps list operations, choose stable operation ids, and extract YAML or JSON through the openapi-extract CLI.
---

# OpenAPI Extract

Use the `openapi-extract` CLI when a task includes an OpenAPI spec file, URL, or pasted OpenAPI document and only a subset of endpoints is needed.

Do not inspect or summarize a large OpenAPI file directly as the first step. First use `openapi-extract` to get a compact operation catalog, then extract the narrowest mini spec that matches the user's request. This keeps the agent context focused and prevents unrelated endpoints from influencing code generation or API reasoning.

Install the CLI if it is missing:

```bash
go install github.com/korECM/openapi-extract@latest
```

Do not read a large OpenAPI file directly unless the user explicitly asks for full-spec analysis. Prefer the two-step agent flow:

```bash
openapi-extract list <openapi.yaml|openapi.json|url|-> --format json
openapi-extract extract <openapi.yaml|openapi.json|url|-> --id '<operation-id>' --stdout
```

If `openapi-extract` is not on `PATH` and you are inside this repository, use:

```bash
go run . list <openapi.yaml|openapi.json|url|-> --format json
go run . extract <openapi.yaml|openapi.json|url|-> --id '<operation-id>' --stdout
```

## Workflow

1. Run `list --format json` to get operation ids, methods, paths, summaries, operationIds, and tags.
2. Pick the narrowest operation ids that answer the user's task.
3. Run `extract --stdout` and pass that mini spec to the next reasoning or code-generation step.
4. Use `--format json` only when the downstream consumer specifically needs JSON. YAML is the default and preferred prompt format.
5. Use `--copy` only for interactive user workflows. For agents, prefer `--stdout`.
6. If the user provides a URL, pass the URL directly to `list` and `extract`; do not download the full spec into the prompt.
7. If the task genuinely requires broad API coverage, select multiple operation ids and extract a combined mini spec instead of reading the whole source spec.

## Commands

List operations:

```bash
openapi-extract list ./openapi.yaml --format json
openapi-extract list https://docs.example.com/openapi.yaml --format json
cat openapi.yaml | openapi-extract list - --format json
```

Extract by stable operation id:

```bash
openapi-extract extract ./openapi.yaml --id 'get_/planets/{planetId}' --stdout
```

Extract multiple operations:

```bash
openapi-extract extract ./openapi.yaml \
  --id 'get_/planets' \
  --id 'get_/planets/{planetId}' \
  --stdout
```

Extract by human-readable selector:

```bash
openapi-extract extract ./openapi.yaml --select 'GET /planets/{planetId}' --stdout
```

## Output Contract

The extracted mini spec keeps selected paths and methods, path-level parameters, reachable `$ref` components, referenced `securitySchemes`, and required root metadata. It removes unrelated paths and unused components.

## Error Handling

- If `operation not found` appears, rerun `list --format json` and select an exact `id`.
- If parsing fails, confirm the input is OpenAPI 3.x YAML or JSON.
- If the output target is missing in non-interactive mode, add `--stdout`, `--copy`, or `--output <file>`.
