# Claude Code Instructions

Use the packaged plugin at `plugins/openapi-extract` for reusable Claude Code workflows.

For project-local OpenAPI extraction tasks, prefer:

```bash
openapi-extract list <openapi.yaml|-> --format json
openapi-extract extract <openapi.yaml|-> --id '<operation-id>' --stdout
```

If the CLI is not installed, run from this repository with `go run .`.
