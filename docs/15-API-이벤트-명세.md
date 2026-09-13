---
title: API · 이벤트 · Discord 명령 명세
type: spec
project: 나만의 Agent 만들기
status: draft v0.1
created: 2026-09-11
tags: [api, rest, websocket, redis-stream, discord, openapi]
---

# API · 이벤트 · Discord 명령 명세

> [!tip] 핵심 Takeaway
> 외부 계약은 네 가지다. 웹 UI가 쓰는 **공개 REST + WebSocket**, Control Plane과 Agent Runtime 사이의 **내부 HTTP**, Agent Runtime이 발행하는 **이벤트 스트림**, 사용자가 쓰는 **Discord 명령·버튼**. 이 문서가 그 네 가지의 단일 기준이며, REST는 springdoc OpenAPI로, 이벤트는 zod/Pydantic 스키마로 코드에서 다시 생성한다.

← [[14-Backend-설계]] · 다음: [[16-구현-로드맵]]

## 1. 공개 REST API (Control Plane, `/api/v1`)

공통: JSON, `Authorization: Bearer <jwt>`, 오류는 `{ "code": "TASK_NOT_FOUND", "message": "...", "details": {} }`, 목록은 `{ "items": [], "next_cursor": "..." }` 커서 페이지네이션.

### Projects

| 메서드 | 경로 | 설명 |
|---|---|---|
| GET | `/projects` | 목록 |
| POST | `/projects` | `{name, github_owner, github_repo, default_branch?, installation_id}` |
| GET | `/projects/{id}` | 상세 (+ `.devsquad` 존재 여부, pipeline 요약) |
| POST | `/projects/{id}/bootstrap` | `.devsquad/` 기본 템플릿을 repo에 커밋 |
| GET | `/projects/{id}/agents/{role}/files` | `{files: [{path, content, sha}]}` |
| PUT | `/projects/{id}/agents/{role}/files/{path}` | `{content, sha, message}` → GitHub 커밋, 새 sha 반환 |
| GET | `/projects/{id}/pipeline` | `pipeline.yaml` 파싱 결과 |

### Tasks

| 메서드 | 경로 | 설명 |
|---|---|---|
| GET | `/tasks?project_id=&status=&cursor=` | 목록. 각 항목에 `stages[]` 요약, `pending_approvals`, `usage` 포함 |
| POST | `/tasks` | `{project_id, command, options?: {token_budget?}}` → 201 `{id, status: "queued"}` |
| GET | `/tasks/{id}` | 상세 (아래 스키마) |
| POST | `/tasks/{id}/pause` · `/resume` · `/cancel` | 상태 전이. 불가한 전이는 409 |
| GET | `/tasks/{id}/events?after_seq=&limit=` | 이벤트 replay (seq 오름차순, 기본 500) |
| GET | `/tasks/{id}/approvals?status=` | 승인 목록 |
| GET | `/tasks/{id}/deliverables` | 산출물 목록 |
| GET | `/tasks/{id}/usage` | 토큰·비용 집계 (stage·agent·model별) |

`GET /tasks/{id}` 응답:

```json
{
  "id": "…", "project_id": "…", "command": "회원 탈퇴 기능 추가 …",
  "status": "waiting_approval",
  "created_at": "…", "updated_at": "…",
  "discord_thread_url": "https://discord.com/channels/…",
  "stages": [
    { "key": "planning", "role": "planner", "status": "approved", "depends_on": [], "retry_count": 1 },
    { "key": "design",   "role": "designer", "status": "deliverable_review", "depends_on": ["planning"], "retry_count": 0 },
    { "key": "frontend", "role": "frontend", "status": "pending", "depends_on": ["design"], "retry_count": 0 },
    { "key": "backend",  "role": "backend",  "status": "pending", "depends_on": ["design"], "retry_count": 0 },
    { "key": "review",   "role": "reviewer", "status": "pending", "depends_on": ["frontend","backend"], "retry_count": 0 },
    { "key": "pr",       "role": "publisher","status": "pending", "depends_on": ["review"], "retry_count": 0 }
  ],
  "pending_approvals": [ { "id": "…", "kind": "deliverable", "stage_key": "design", "title": "designer deliverable v1", "requested_at": "…" } ],
  "usage": { "input_tokens": 182340, "output_tokens": 41200, "budget": 2000000 },
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

응답 200 `{ id, status, decided_at, decided_via: "web" }`. 이미 결정됨 409 `{code: "APPROVAL_ALREADY_DECIDED", details: {decided_via, decided_at}}`. reject에 feedback 누락 400.

### Deliverables

| 메서드 | 경로 | 설명 |
|---|---|---|
| GET | `/deliverables/{id}` | `{id, task_id, stage_key, kind, content?, uri?, commit_sha?, summary, created_at}` |
| GET | `/deliverables/{id}/diff?base=` | kind=diff/pr일 때 unified diff 텍스트 |

### 기타

`GET /me`, `GET /health`, `GET /integrations/status` → `{discord: "connected", github: "installed", agent_runtime: "healthy", redis: "ok"}`.

## 2. WebSocket (`/ws`)

연결 `wss://host/ws?token=<jwt>`. 클라이언트 → 서버:

```json
{ "op": "subscribe",   "task_id": "…", "from_seq": 1230 }
{ "op": "subscribe" }                       // 요약 채널: 내 모든 Task의 approval.* / run.*
{ "op": "unsubscribe", "task_id": "…" }
{ "op": "ping" }
```

서버 → 클라이언트: 이벤트 envelope(§3) 그대로, 추가로 `{ "op": "pong" }`, `{ "op": "subscribed", "task_id", "replayed": 42 }`, `{ "op": "error", "code": "TASK_FORBIDDEN" }`.

## 3. 이벤트 스키마 (Redis Stream `devsquad:events` · WS 공용)

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
| `run.started` | `{}` | AR |
| `run.paused` | `{reason: "user" \| "budget"}` | CP(사용자) / AR(예산) |
| `run.resumed` | `{}` | AR |
| `run.completed` | `{pr_urls?: []}` | AR |
| `run.failed` | `{error, node?}` | AR |
| `run.cancelled` | `{}` | CP |
| `stage.started` | `{}` | AR |
| `stage.completed` | `{deliverable_id}` | AR |
| `stage.blocked` | `{reason, last_feedback}` | AR |
| `agent.thinking` | `{summary}` (≤ 200자) | AR |
| `agent.tool_call` | `{call_id, tool, args_summary}` | AR |
| `agent.tool_result` | `{call_id, tool, ok, summary, duration_ms}` | AR |
| `agent.message` | `{text}` | AR |
| `approval.requested` | `{approval_id, kind, retry_no, title, content_preview, deliverable_id?}` | CP(AR의 interrupt를 받아 approval_id 부여 후 재발행) |
| `approval.decided` | `{approval_id, decision, decided_via, decided_by}` | CP |
| `deliverable.produced` | `{deliverable_id, kind, summary, commit_sha?}` | CP(AR의 원본을 저장 후 id 부여) |
| `usage` | `{model, input_tokens, output_tokens}` | AR |
| `pr.opened` | `{url, number, role}` | CP |
| `pr.review_comment` | `{url, author, body_preview}` | CP(webhook) |

AR이 발행하는 원본 이벤트 중 `approval.requested`와 `deliverable.produced`는 CP가 DB id를 붙여 **재발행**하며 WS에는 재발행본만 나간다. 그 외는 CP가 저장 후 그대로 전달한다.

## 4. 내부 HTTP (CP ↔ AR, `X-Internal-Token`)

### CP → AR (`http://agent-runtime:8100`)

| 메서드 | 경로 | 요청 | 응답 |
|---|---|---|---|
| POST | `/runs` | `{task_id, project: {owner, repo, branch, installation_token, context_path}, command, options}` | 202 `{task_id, thread_id}` |
| POST | `/runs/{task_id}/resume` | `{approval_id, decision, feedback?, edited_content?}` | 202 |
| POST | `/runs/{task_id}/continue` | `{}` | 202 |
| POST | `/runs/{task_id}/cancel` | `{}` | 200 |
| GET | `/runs/{task_id}/state` | | `{next: [], interrupts: [{id, value}], active: bool, checkpoint_id, values_summary}` |
| GET | `/health` | | `{status, active_runs}` |

`installation_token`은 CP가 발급한 1시간 GitHub 토큰이다. AR은 이를 workspace clone·push에만 쓴다.

### AR → CP (`http://control-plane:8080/internal`)

| 메서드 | 경로 | 요청 | 설명 |
|---|---|---|---|
| POST | `/internal/prs` | `{task_id, role, head_branch, title, body}` | PR 생성(기존 PR 있으면 반환). 응답 `{url, number}` |
| POST | `/internal/github-token` | `{task_id}` | 토큰 만료 시 재발급 |

## 5. Discord 명령과 인터랙션

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
| `RUNTIME_UNAVAILABLE` | 503 | AR 헬스 실패. Task는 queued로 유지, 재시도 |
| `BUDGET_EXCEEDED` | 409 | 예산 초과로 paused인 Task resume 시 예산 증액 필요 |
