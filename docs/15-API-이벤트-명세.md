---
title: API · 이벤트 · Discord 명령 명세
type: spec
project: 나만의 Agent 만들기
status: draft v0.1
created: 2026-09-11
tags: [api, rest, websocket, redis-stream, discord, openapi]
---

# API · 이벤트 · Discord 명령 명세

> **현재 구현 (2026-10-06):** [19 · 통합 서비스 설계](19-Go-통합-서비스-설계.md)에 따라 `/api/v1`과 `/ws`는 단일 Go 서비스가 제공한다. 공개 계약은 유지하며 `/runs/*`, CP↔AR HTTP, Redis 이벤트 전송은 제거했다. OpenAPI는 `/openapi.json`, 문서 UI는 `/docs`, 지표는 `/metrics`에서 제공한다.

> [!tip] 핵심 Takeaway
> 현재 외부 계약은 웹 UI의 **공개 REST + WebSocket**이다. 이벤트 원장은 PostgreSQL에 있으며 Discord 명령·버튼은 후속 설계다. 현재 구현은 Go의 명시적 JSON 타입과 웹의 zod 스키마로 계약을 검증한다.

← [[14-Backend-설계]] · 다음: [[16-구현-로드맵]]

## 1. 공개 REST API (`/api/v1`)

공통: snake_case JSON, 오류는 `{ "code": "TASK_NOT_FOUND", "message": "...", "details": {} }`. 현재 로컬 개발 API에는 JWT 인증을 적용하지 않는다. Task 목록은 `{items, next_cursor: null}`, 프로젝트·승인 목록은 배열이다. 실제 제공 엔드포인트와 스키마는 `/openapi.json`이 기준이다. 아래에 후속 기능을 따로 표시한다.

### Projects

| 메서드 | 경로 | 설명 |
|---|---|---|
| GET | `/projects` | 목록 |
| POST | `/projects` | `{name, github_owner, github_repo, default_branch?, installation_id?, local_path?}` |
| GET | `/projects/{id}` | 상세. `.devsquad` 요약은 후속 |
| POST | `/projects/{id}/bootstrap` | 후속: `.devsquad/` 기본 템플릿을 repo에 커밋 |
| GET | `/projects/{id}/agents/{role}/files` | 후속: `{files: [{path, content, sha}]}` |
| PUT | `/projects/{id}/agents/{role}/files/{path}` | 후속: `{content, sha, message}` → GitHub 커밋, 새 sha 반환 |
| GET | `/projects/{id}/pipeline` | 후속: `pipeline.yaml` 파싱 결과 |

### Tasks

| 메서드 | 경로 | 설명 |
|---|---|---|
| GET | `/tasks?project_id=&status=` | 목록. `stages[]`, `pending_approvals`, `last_event` 포함 |
| POST | `/tasks` | `{project_id, command, options?}` → 201 Task 상세. options는 호환용이며 예산 정책은 후속 |
| GET | `/tasks/{id}` | 상세 (아래 스키마) |
| POST | `/tasks/{id}/pause` · `/resume` · `/cancel` | 상태 전이. 불가한 전이는 409 |
| GET | `/tasks/{id}/events?after_seq=&limit=` | 이벤트 replay (seq 오름차순, 기본 500) |
| GET | `/tasks/{id}/approvals?status=` | 승인 목록 |
| GET | `/tasks/{id}/deliverables` | 후속: 산출물 목록 |
| GET | `/tasks/{id}/usage` | 후속: 토큰·비용 집계 (stage·agent·model별) |

`GET /tasks/{id}` 응답:

```json
{
  "id": "…", "project_id": "…", "command": "회원 탈퇴 기능 추가 …",
  "status": "waiting_approval",
  "created_at": "…", "updated_at": "…",
  "discord_thread_id": null,
  "stages": [
    { "key": "planning", "role": "planner", "status": "approved", "depends_on": [], "retry_count": 1 },
    { "key": "design",   "role": "designer", "status": "deliverable_review", "depends_on": ["planning"], "retry_count": 0 },
    { "key": "frontend", "role": "frontend", "status": "pending", "depends_on": ["design"], "retry_count": 0 },
    { "key": "backend",  "role": "backend",  "status": "pending", "depends_on": ["design"], "retry_count": 0 },
    { "key": "review",   "role": "reviewer", "status": "pending", "depends_on": ["frontend","backend"], "retry_count": 0 },
    { "key": "pr",       "role": "publisher","status": "pending", "depends_on": ["review"], "retry_count": 0 }
  ],
  "pending_approvals": [ { "id": "…", "kind": "deliverable", "stage_key": "design", "title": "designer deliverable v1", "requested_at": "…" } ],
  "last_event": { "seq": 412, "type": "approval.requested", "ts": "…" }
}
```

### Approvals

| 메서드 | 경로 | 설명 |
|---|---|---|
| GET | `/approvals?status=pending` | 사용자 전체 대기 승인 (헤더 배지용) |
| GET | `/approvals/{id}` | 원문 포함 상세 |
| POST | `/approvals/{id}/decide` | 아래 |

`POST /approvals/{id}/decide` 요청:

```json
{ "decision": "approve" }
{ "decision": "reject", "feedback": "유예 기간 중 로그인 시 복구 안내 모달 필요" }
{ "decision": "edit",   "edited_content": "# 수정된 Plan …" }
```

응답 200 `{ id, status, decided_at, decided_via: "web" }`. 이미 결정됨 409 `{code: "APPROVAL_ALREADY_DECIDED", details: {}}`. reject에 feedback 누락 400.

### Deliverables (후속)

| 메서드 | 경로 | 설명 |
|---|---|---|
| GET | `/deliverables/{id}` | `{id, task_id, stage_key, kind, content?, uri?, commit_sha?, summary, created_at}` |
| GET | `/deliverables/{id}/diff?base=` | kind=diff/pr일 때 unified diff 텍스트 |

### 기타

`GET /health`는 `{status: "ok", service: "devsquad", ts}`를 반환하고 DB 연결을 점검한다. `/me`, `/integrations/status`는 후속 범위다.

## 2. WebSocket (`/ws`)

현재 연결은 `ws://localhost:8080/ws`이며 JWT 검증은 후속이다. 클라이언트 → 서버:

```json
{ "op": "subscribe",   "task_id": "…", "from_seq": 1230 }
{ "op": "subscribe" }                       // 요약 채널: 모든 Task의 approval.* / run.* / stage.*
{ "op": "unsubscribe", "task_id": "…" }
{ "op": "ping" }
```

서버 → 클라이언트: 이벤트 envelope(§3) 그대로, 추가로 `{ "op": "pong" }`, `{ "op": "subscribed", "task_id", "replayed": 42 }`, `{ "op": "error", "code": "BAD_REQUEST" }`.

## 3. 이벤트 스키마 (PostgreSQL `task_event` · WS 공용)

```json
{
  "event_id": "uuid",
  "task_id": "uuid",
  "seq": 1234,
  "ts": "2026-09-11T10:00:00.000Z",
  "stage_key": "frontend",
  "agent": "frontend",
  "type": "agent.tool_call",
  "payload": { }
}
```

| type | payload 필드 | 발행 주체 |
|---|---|---|
| `run.started` | `{stages, levels}` | orchestrator / stage |
| `run.paused` | `{reason: "user" \| "budget"}` | app (예산 강제는 후속) |
| `run.resumed` | `{}` | orchestrator / stage |
| `run.completed` | `{pr_urls?: []}` | orchestrator / stage |
| `run.failed` | `{error, node?}` | orchestrator / stage |
| `run.cancelled` | `{}` | app |
| `stage.started` | `{}` | orchestrator / stage |
| `stage.completed` | `{deliverable_id}` | orchestrator / stage |
| `stage.blocked` | `{reason, last_feedback}` | orchestrator / stage |
| `agent.thinking` | `{summary}` (≤ 200자) | orchestrator / stage |
| `agent.tool_call` | `{call_id, tool, args_summary}` | orchestrator / stage |
| `agent.tool_result` | `{call_id, tool, ok, summary, duration_ms}` | orchestrator / stage |
| `agent.message` | `{text}` | orchestrator / stage |
| `approval.requested` | `{approval_id, kind, retry_no, title, content_preview, deliverable_id?}` | app / orchestrator |
| `approval.decided` | `{approval_id, decision, decided_via, decided_by}` | app |
| `deliverable.produced` | `{deliverable_id, kind, summary, commit_sha?}` | app / orchestrator |
| `usage` | `{model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens}` | orchestrator / stage |
| `pr.opened` | `{url, number, role}` | 후속 구현 |
| `pr.review_comment` | `{url, author, body_preview}` | 후속 구현 |

모든 이벤트는 `task.event_seq` 증가와 `task_event` INSERT를 같은 트랜잭션에서 처리한다. `approval.requested`와 `deliverable.produced`에는 생성 시점부터 DB id가 들어간다. 커밋 후 Bus가 WS에 전달하며 REST 재생과 같은 envelope을 사용한다. 느린 구독자는 연결을 종료하고 `from_seq`로 복구한다.

## 4. 내부 실행

외부 Runtime HTTP는 제공하지 않는다. Task 생성·승인·resume 커밋 후 Reconcile을 깨우며, 시작 시와 고정 지연 주기로 DB 상태를 다시 읽는다. 기존 `/runs/*`, `/internal/*`, `X-Internal-Token`은 이 구현의 계약에서 제외한다. Discord·GitHub API는 후속 범위다.

## 5. Discord 명령과 인터랙션 (후속 설계)

### Slash commands (guild 등록)

| 명령 | 옵션 | 동작 |
|---|---|---|
| `/task` | `command: string` (필수), `project: choice` (기본 프로젝트 있으면 생략) | Task 생성, 채널에 thread 생성, "접수" 메시지 |
| `/status` | `task: autocomplete` (생략 시 현재 thread의 Task) | 파이프라인 요약 Embed |
| `/pause` · `/resume` · `/cancel` | `task` | 상태 전이 |
| `/approvals` | | 내 대기 승인 목록 + 웹 링크 |

모든 Interaction은 3초 내 `deferReply(ephemeral=true)` 후 처리.

### 승인 카드 (Embed)

```
┌ 📝 Plan · 기획 agent · v1 ─────────────────────┐
│ Task: 회원 탈퇴 기능 추가                          │
│ ─────────────────────────────────────────────  │
│ 1. 요구사항 정의서 docs/spec/withdrawal.md 작성  │
│ 2. 유스케이스 3개 (탈퇴 신청 / 유예 중 복구 /   │
│    유예 만료 처리)                                │
│ 3. 화면 목록 초안                                 │
│ … (1,500자 초과 시 잘림) · 🔗 웹에서 전체 보기    │
│ ─────────────────────────────────────────────  │
│ [✅ 승인]  [✏️ 수정 (웹)]  [❌ 반려]              │
│ 요청 10:01 · 대기 중                              │
└────────────────────────────────────────────────┘
```

버튼 customId: `approve:{approval_id}`, `edit:{approval_id}`(웹 링크 응답), `reject:{approval_id}`(Modal: `feedback` 텍스트 필수). 결정 후 버튼 비활성화, footer를 "✅ 승인됨 · Jin Ho · 10:05 (Discord)"로 갱신. 웹에서 결정되면 "(웹)"으로 표기.

### thread 알림

Stage 시작·완료, 반려 재제출, blocked, PR 링크, 예산 초과 일시정지. `agent.thinking`·`tool_call`은 Discord에 보내지 않는다(소음).

## 6. `pipeline.yaml` 스키마

```yaml
version: 1                       # 정수, 필수
stages:                          # 1개 이상, DAG여야 함
  - id: string                   # 고유, [a-z_]+
    agent: string                # .devsquad/agents/<agent>/ 존재해야 함
    depends_on: [string]         # 기본 []
    approvals: [plan|deliverable]  # 기본 [plan, deliverable]
    tools: [string]              # 선택, 기본은 역할 기본 화이트리스트
    model: string                # 선택, 예: anthropic/claude-sonnet-4-5
policy:
  max_retries_per_approval: int  # 기본 3
  token_budget: int              # 기본 project.token_budget
  execute_max_iterations: int    # 기본 40
```

검증 실패(사이클, 없는 agent)는 Task 생성 시 400으로 반환한다.

## 7. 오류 코드

| code | HTTP | 상황 |
|---|---|---|
| `TASK_NOT_FOUND` / `APPROVAL_NOT_FOUND` | 404 | |
| `APPROVAL_ALREADY_DECIDED` | 409 | 다른 채널에서 먼저 결정 |
| `INVALID_TRANSITION` | 409 | 예: completed Task에 pause |
| `FEEDBACK_REQUIRED` | 400 | reject에 피드백 없음 |
| `PIPELINE_INVALID` | 400 | yaml 검증 실패 |
| `PROJECT_INVALID` | 400 | local_path 또는 workspace 준비 실패 |
| `EDITED_CONTENT_REQUIRED` | 400 | edit에 edited_content 없음 |
| `BUDGET_EXCEEDED` | 409 | 예산 초과로 paused인 Task resume 시 예산 증액 필요 |
