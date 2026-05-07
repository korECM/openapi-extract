---
name: openapi-extract
description: Use when an AI agent needs a small OpenAPI 3.x mini spec for selected endpoints without reading an entire OpenAPI document. Helps list operations, choose stable operation ids, and extract YAML or JSON through the openapi-extract CLI.
---

# OpenAPI Extract

Use the `openapi-extract` CLI when a task needs a compact OpenAPI spec for a few endpoints.

Do not read a large OpenAPI file directly unless the user explicitly asks. Prefer the two-step agent flow:

```bash
openapi-extract list <openapi.yaml|-> --format json
openapi-extract extract <openapi.yaml|-> --id '<operation-id>' --stdout
```

If `openapi-extract` is not on `PATH` and you are inside this repository, use:

```bash
go run . list <openapi.yaml|-> --format json
go run . extract <openapi.yaml|-> --id '<operation-id>' --stdout
```

## Workflow

1. Run `list --format json` to get operation ids, methods, paths, summaries, operationIds, and tags.
2. Pick the narrowest operation ids that answer the user's task.
3. Run `extract --stdout` and pass that mini spec to the next reasoning or code-generation step.
4. Use `--format json` only when the downstream consumer specifically needs JSON. YAML is the default and preferred prompt format.
5. Use `--copy` only for interactive user workflows. For agents, prefer `--stdout`.

## Commands

List operations:

```bash
openapi-extract list ./openapi.yaml --format json
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
