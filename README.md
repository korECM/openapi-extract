# openapi-extract

`openapi-extract` is a Go TUI and CLI for turning a large OpenAPI 3.x document into a small, AI-friendly mini spec.

It is built for two workflows:

- Humans can open a terminal UI, search operations, select a few endpoints, then copy or save the extracted spec.
- AI agents and scripts can list operation IDs first, then extract only the operations they need without reading the full OpenAPI file directly.

## Install / Build

```bash
go build ./...
go build -o openapi-extract .
```

## Interactive TUI

```bash
openapi-extract ./openapi.yaml
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

## CLI Reference

List operations:

```bash
openapi-extract list <openapi.yaml|-> [--format text|json]
```

Extract selected operations:

```bash
openapi-extract extract <openapi.yaml|-> \
  (--id ID | --select 'METHOD /path') \
  (--stdout | --copy | --output mini.openapi.yaml) \
  [--format yaml|json]
```

Input sources:

- Local file: `openapi-extract list ./openapi.yaml`
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

## Korean / 한국어

`openapi-extract`는 큰 OpenAPI 3.x 문서에서 필요한 endpoint만 골라 작은 AI-friendly spec으로 추출하는 Go TUI/CLI 도구입니다.

두 가지 사용 흐름을 지원합니다.

- 사람이 쓸 때는 TUI에서 operation을 검색하고 여러 개 선택한 뒤, 클립보드로 복사하거나 파일로 저장합니다.
- AI agent나 스크립트가 쓸 때는 먼저 operation catalog를 조회하고, 필요한 operation id만 넘겨 mini spec을 추출합니다.

## 빌드

```bash
go build ./...
go build -o openapi-extract .
```

## TUI 사용법

```bash
openapi-extract ./openapi.yaml
cat openapi.yaml | openapi-extract -
```

단축키:

- `j/k` 또는 방향키: 이동
- `/`: method, path, tag, summary, operationId 검색
- `space`: operation 선택/해제
- `c`: 선택한 operation의 mini spec을 클립보드로 복사
- `s`: 선택한 operation의 mini spec을 파일로 저장
- `y`: 선택한 operation의 mini spec을 stdout으로 출력하고 종료
- `q` 또는 `esc`: 종료

## AI Agent Workflow

agent는 OpenAPI 원문 전체를 읽기보다 `list`로 작은 catalog를 먼저 받는 흐름을 권장합니다.

```bash
openapi-extract list /Users/devsisters/Downloads/scalar-galaxy.yaml --format json
```

예시 text catalog:

```text
post_/auth/token              POST /auth/token - Get a token
get_/me                       GET /me - Get authenticated user
get_/planets                  GET /planets - Get all planets
get_/planets/{planetId}       GET /planets/{planetId} - Get a planet
```

그 다음 `id`로 추출합니다.

```bash
openapi-extract extract /Users/devsisters/Downloads/scalar-galaxy.yaml \
  --id 'get_/planets/{planetId}' \
  --stdout
```

여러 operation도 한 번에 추출할 수 있습니다.

```bash
openapi-extract extract /Users/devsisters/Downloads/scalar-galaxy.yaml \
  --id 'get_/planets' \
  --id 'get_/planets/{planetId}' \
  --stdout
```

사람이 직접 입력하기 쉬운 selector도 지원합니다.

```bash
openapi-extract extract /Users/devsisters/Downloads/scalar-galaxy.yaml \
  --select 'GET /planets/{planetId}' \
  --stdout
```

## CLI 레퍼런스

operation 목록 출력:

```bash
openapi-extract list <openapi.yaml|-> [--format text|json]
```

선택한 operation 추출:

```bash
openapi-extract extract <openapi.yaml|-> \
  (--id ID | --select 'METHOD /path') \
  (--stdout | --copy | --output mini.openapi.yaml) \
  [--format yaml|json]
```

입력:

- 로컬 파일: `openapi-extract list ./openapi.yaml`
- stdin: `cat openapi.yaml | openapi-extract list - --format json`

출력:

- `--stdout`: mini spec을 터미널에 출력
- `--copy`: mini spec을 클립보드에 복사
- `--output <path>`: mini spec을 파일로 저장

## 추출 범위

mini spec에는 선택한 operation을 쓰는 데 필요한 최소 항목만 남깁니다.

- 선택한 `paths`와 HTTP method
- 선택한 path의 path-level parameter
- 선택 operation에서 재귀적으로 도달 가능한 `$ref` components
- 참조되는 `securitySchemes`
- 필요한 root `openapi`, `info`, `servers`, `security`, `tags`, `externalDocs`

관련 없는 path와 사용하지 않는 component를 제거해서 LLM prompt에 넣기 좋은 작은 spec을 만듭니다.
