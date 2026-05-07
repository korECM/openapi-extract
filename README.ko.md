# openapi-extract

[English README](README.md)

<p align="center">
  <video src="./introduction/out/openapi-extract-intro-1080p.mp4" controls width="100%"></video>
</p>

[30초 소개 영상 보기](introduction/out/openapi-extract-intro-1080p.mp4)

`openapi-extract`는 큰 OpenAPI 3.x 문서에서 필요한 endpoint만 골라 작은 AI-friendly mini spec으로 추출하는 Go TUI/CLI 도구입니다.

LLM에게 전체 API 계약을 붙여넣지 않아도 됩니다. 필요한 것이 `GET /players/{id}` 하나라면, operation catalog를 먼저 보고 필요한 operation만 골라 `$ref`까지 살아 있는 작은 OpenAPI spec으로 만들 수 있습니다.

왜 필요한가:

- prompt가 작아집니다. Scalar Galaxy 예시에서 단일 operation은 44,250 bytes / 1,450 lines에서 14,837 bytes / 514 lines로 줄어듭니다. byte 기준 약 66% 감소입니다.
- agent가 더 집중합니다. 전체 spec을 읽는 대신 operation catalog를 보고 안정적인 operation id로 추출합니다.
- mini spec이 깨지지 않습니다. 선택 operation에 필요한 schema, response, parameter, header, request body, security scheme을 함께 보존합니다.
- 사람도 편합니다. TUI에서 검색, multi-select, copy, save를 할 수 있어 YAML을 손으로 잘라낼 필요가 없습니다.

두 가지 사용 흐름을 지원합니다.

- 사람이 쓸 때는 TUI에서 operation을 검색하고 여러 개 선택한 뒤, 클립보드로 복사하거나 파일로 저장합니다.
- AI agent나 스크립트가 쓸 때는 먼저 operation catalog를 조회하고, 필요한 operation id만 넘겨 mini spec을 추출합니다.

## 설치 / 빌드

최신 커밋을 설치합니다.

```bash
go install github.com/korECM/openapi-extract@latest
```

Go bin 경로가 `PATH`에 들어 있어야 합니다.

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

로컬 checkout에서 빌드합니다.

```bash
go build ./...
go build -o openapi-extract .
```

## TUI 사용법

```bash
openapi-extract ./openapi.yaml
openapi-extract https://docs.example.com/openapi.yaml
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
openapi-extract list https://docs.example.com/openapi.yaml --format json
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

출력은 여전히 OpenAPI 문서입니다. 단지 작아집니다.

```text
Scalar Galaxy 전체 spec:              44,250 bytes / 1,450 lines
GET /planets/{planetId} mini spec:    14,837 bytes /   514 lines
감소율:                               byte 기준 약 66%
```

## Agent 연동

여러 coding agent가 같은 추출 흐름을 쓰도록 plugin/rule/skill 파일을 포함했습니다.

- Codex: `plugins/openapi-extract/.codex-plugin/plugin.json`, `.agents/plugins/marketplace.json`
- Claude Code: `plugins/openapi-extract/.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`
- Cursor: `.cursor/rules/openapi-extract.mdc`
- OpenCode와 범용 agent: `AGENTS.md`
- 공통 skill: `skills/openapi-extract/SKILL.md`

빠른 설치:

```bash
# Claude Code plugin marketplace
/plugin marketplace add korECM/openapi-extract
/plugin install openapi-extract@openapi-extract-marketplace

# Agent Skills CLI
npx skills add korECM/openapi-extract --skill openapi-extract
```

설치와 사용 예시는 [docs/agent-integrations.md](docs/agent-integrations.md)를 참고하세요.

## CLI 레퍼런스

operation 목록 출력:

```bash
openapi-extract list <openapi.yaml|url|-> [--format text|json] [--columns id,method,path,summary] [--no-header] [--no-color]
```

text 출력은 컬럼 선택을 지원합니다.

```bash
openapi-extract list ./openapi.yaml --columns method,path,tags,operationId
openapi-extract list ./openapi.yaml --columns all
```

선택한 operation 추출:

```bash
openapi-extract extract <openapi.yaml|url|-> \
  (--id ID | --select 'METHOD /path') \
  (--stdout | --copy | --output mini.openapi.yaml) \
  [--format yaml|json]
```

입력:

- 로컬 파일: `openapi-extract list ./openapi.yaml`
- URL: `openapi-extract list https://docs.example.com/openapi.yaml --format json`
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
