# Agent Instructions

This repository provides `openapi-extract`, a Go CLI/TUI for creating small OpenAPI 3.x mini specs from selected operations.

When an agent needs OpenAPI context, prefer this flow instead of reading a large OpenAPI document directly:

```bash
openapi-extract list <openapi.yaml|-> --format json
openapi-extract extract <openapi.yaml|-> --id '<operation-id>' --stdout
```

If the binary is not installed and you are working from this repository, use:

```bash
go run . list <openapi.yaml|-> --format json
go run . extract <openapi.yaml|-> --id '<operation-id>' --stdout
```

Use `--stdout` for AI-agent workflows. Use `--copy` only when the user wants clipboard behavior, and `--output <file>` only when the user asks to save a mini spec.

Validation commands:

```bash
GOCACHE=/private/tmp/openapi-extract-go-cache go test ./...
GOCACHE=/private/tmp/openapi-extract-go-cache go build ./...
GOCACHE=/private/tmp/openapi-extract-go-cache go vet ./...
```
