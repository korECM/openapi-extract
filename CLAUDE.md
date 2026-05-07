# Claude Code Instructions

Use the packaged plugin at `plugins/openapi-extract` for reusable Claude Code workflows.

Install the CLI with:

```bash
go install github.com/korECM/openapi-extract@latest
```

For project-local OpenAPI extraction tasks, prefer:

```bash
openapi-extract list <openapi.yaml|url|-> --format json
openapi-extract extract <openapi.yaml|url|-> --id '<operation-id>' --stdout
```

If the CLI is not installed, run from this repository with `go run .`.
