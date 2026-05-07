# Agent Integrations

This guide explains how to use `openapi-extract` from Codex, Claude Code, Cursor, OpenCode, and other coding agents.

## Shared Agent Flow

Agents should avoid reading large OpenAPI documents directly. First ask the CLI for a compact operation catalog, then extract by operation id.

Install:

```bash
go install github.com/korECM/openapi-extract@latest
```

Ensure `$(go env GOPATH)/bin` is on `PATH`, then run:

```bash
openapi-extract list /Users/devsisters/Downloads/scalar-galaxy.yaml --format json
openapi-extract extract /Users/devsisters/Downloads/scalar-galaxy.yaml --id 'get_/planets/{planetId}' --stdout
openapi-extract list https://docs.example.com/openapi.yaml --format json
```

For human-readable discovery, use selectable text columns:

```bash
openapi-extract list ./openapi.yaml --columns method,path,tags,operationId
```

If the binary is not installed and the agent is inside this repository:

```bash
go run . list /Users/devsisters/Downloads/scalar-galaxy.yaml --format json
go run . extract /Users/devsisters/Downloads/scalar-galaxy.yaml --id 'get_/planets/{planetId}' --stdout
```

## Codex

Codex-compatible plugin files are in:

```text
plugins/openapi-extract/.codex-plugin/plugin.json
plugins/openapi-extract/skills/openapi-extract/SKILL.md
.agents/plugins/marketplace.json
```

Use the repo-local marketplace entry when installing through a Codex plugin UI or local plugin registry. The skill teaches Codex to run `list --format json` first and `extract --stdout` second.

## Claude Code

Claude Code plugin files are in:

```text
plugins/openapi-extract/.claude-plugin/plugin.json
plugins/openapi-extract/skills/openapi-extract/SKILL.md
.claude-plugin/marketplace.json
```

Local development:

```bash
claude --plugin-dir ./plugins/openapi-extract
```

Marketplace install flow inside Claude Code:

```text
/plugin marketplace add korECM/openapi-extract
/plugin install openapi-extract@openapi-extract-marketplace
/openapi-extract:openapi-extract
```

You can also use the full git URL:

```text
/plugin marketplace add https://github.com/korECM/openapi-extract.git
```

Claude Code plugin skills are namespaced by plugin name, so the skill command is `/openapi-extract:openapi-extract`.

## Agent Skills CLI

The repository also exposes a root skill for `npx skills`:

```text
skills/openapi-extract/SKILL.md
```

Install it with:

```bash
npx skills add korECM/openapi-extract --skill openapi-extract
```

Target specific agents if needed:

```bash
npx skills add korECM/openapi-extract --skill openapi-extract --agent claude-code --agent cursor
npx skills add korECM/openapi-extract --skill openapi-extract --global -y
```

## Cursor

Cursor project rule:

```text
.cursor/rules/openapi-extract.mdc
```

The rule is not always applied. It should be selected or referenced when working with OpenAPI files or API-client generation tasks.

## OpenCode

OpenCode reads project rules from:

```text
AGENTS.md
```

It also supports Claude Code conventions as fallbacks, so `CLAUDE.md` and Claude skills can help users migrating from Claude Code. For OpenCode tasks, ask it to use the `AGENTS.md` OpenAPI extraction flow.

## Generic Agents

Any agent can use the plain skill file:

```text
skills/openapi-extract/SKILL.md
```

The minimum prompt instruction is:

```text
Use openapi-extract list <spec> --format json first. Then use openapi-extract extract <spec> --id '<operation-id>' --stdout.
```

## 한국어

`openapi-extract`는 agent가 큰 OpenAPI 파일 전체를 읽지 않고 필요한 endpoint만 작은 spec으로 뽑기 위한 도구입니다.

설치:

```bash
go install github.com/korECM/openapi-extract@latest
```

`$(go env GOPATH)/bin`이 `PATH`에 들어 있어야 합니다.

권장 흐름:

```bash
openapi-extract list /Users/devsisters/Downloads/scalar-galaxy.yaml --format json
openapi-extract extract /Users/devsisters/Downloads/scalar-galaxy.yaml --id 'get_/planets/{planetId}' --stdout
openapi-extract list https://docs.example.com/openapi.yaml --format json
```

repo 안에서 binary가 아직 설치되지 않았다면:

```bash
go run . list /Users/devsisters/Downloads/scalar-galaxy.yaml --format json
go run . extract /Users/devsisters/Downloads/scalar-galaxy.yaml --id 'get_/planets/{planetId}' --stdout
```

Codex는 `.agents/plugins/marketplace.json`과 `.codex-plugin/plugin.json`을 사용합니다.

Claude Code는 공식 marketplace 흐름으로 설치할 수 있습니다.

```text
/plugin marketplace add korECM/openapi-extract
/plugin install openapi-extract@openapi-extract-marketplace
```

Agent Skills CLI도 지원합니다.

```bash
npx skills add korECM/openapi-extract --skill openapi-extract
```

Cursor는 `.cursor/rules/openapi-extract.mdc`를 사용합니다.

OpenCode와 범용 agent는 `AGENTS.md`를 사용합니다.
