# Agent Flow Examples

Use this when you need an example to copy into a prompt, rule, or runbook.

## Local File

```bash
openapi-extract list /Users/devsisters/Downloads/scalar-galaxy.yaml --format json
openapi-extract extract /Users/devsisters/Downloads/scalar-galaxy.yaml --id 'get_/planets/{planetId}' --stdout
```

## Stdin

```bash
cat openapi.yaml | openapi-extract list - --format json
cat openapi.yaml | openapi-extract extract - --id 'get_/planets/{planetId}' --stdout
```

## Repository Fallback

```bash
go run . list /Users/devsisters/Downloads/scalar-galaxy.yaml --format json
go run . extract /Users/devsisters/Downloads/scalar-galaxy.yaml --id 'get_/planets/{planetId}' --stdout
```
