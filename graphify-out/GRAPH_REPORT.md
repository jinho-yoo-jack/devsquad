# Graph Report - devsquad  (2026-10-06)

## Corpus Check
- 149 files · ~61,971 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 9 file(s) not represented in the graph (top: (none) 7, .example 1, .css 1)

## Summary
- 952 nodes · 2295 edges · 47 communities (35 shown, 12 thin omitted)
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 94 edges (avg confidence: 0.85)
- Token cost: 450,051 input · 0 output

## Community Hubs (Navigation)
- Go 진입점·HTTP·WS
- 이벤트 Bus·통합 테스트
- Go 포팅·로드맵 설계 문서
- 백엔드 팀원 템플릿
- Task·Approval 앱 서비스
- 기획자 팀원 템플릿
- Orchestrator·Stage 실행기
- 웹 에이전트 활동 패널
- 설정·샌드박스·워크스페이스
- 웹 이벤트 타임라인
- LLM 어댑터
- 웹 Task 목록·생성 화면
- 웹 WS 연결
- 웹 API 클라이언트·쿼리
- Store 엔티티 매핑
- sqlc 쿼리 메서드
- 웹 tsconfig
- 웹 런타임 의존성
- sqlc 파라미터 타입
- 웹 승인 패널·API 스키마
- 웹 Task 상세 화면
- 웹 개발 의존성
- README·기여 규칙
- sqlc 모델
- 웹 앱 레이아웃
- PRD 승인 게이트 개념
- pgx 트랜잭션 계층
- 웹 package 의존성
- CrewAI·LangGraph 기술 정리
- 팀원 정의·파이프라인 개념
- Compose 서비스·CI
- 웹 npm 스크립트
- 이벤트 envelope·seq
- SCM Publisher 자리
- 승인 멱등성·상태 기계
- 재시작 복구 개념
- 디자인 화면 원칙
- 도메인 상태 오류
- Vitest 설정
- Store 커밋 오류
- PRD Task·Stage
- 토큰 예산 훅
- Playwright 설정
- devsquad YAML 블록
- MVP 범위
- Go 모듈

## God Nodes (most connected - your core abstractions)
1. `Orchestrator` - 28 edges
2. `Go 통합 서비스 설계` - 28 edges
3. `Queries` - 22 edges
4. `newHarness()` - 19 edges
5. `harness` - 18 edges
6. `WsConnection` - 18 edges
7. `TaskEvent` - 16 edges
8. `compilerOptions` - 16 edges
9. `TaskDetailPage()` - 15 edges
10. `구현 로드맵 — Phase 1~3 작업 분해` - 15 edges

## Surprising Connections (you probably didn't know these)
- `DevSquad (approval-based AI dev team tool)` --cites--> `DevSquad PRD`  [EXTRACTED]
  README.md → docs/10-PRD.md
- `Go single service + PostgreSQL backend` --conceptually_related_to--> `Control Plane (Spring Boot, initial design)`  [INFERRED]
  README.md → docs/11-시스템-아키텍처.md
- `DB schema (project, task, stage, approval, deliverable, task_event, usage_record)` --conceptually_related_to--> `sqlc codegen config (pgx/v5)`  [AMBIGUOUS]
  docs/14-Backend-설계.md → sqlc.yaml
- `run()` --calls--> `Load()`  [EXTRACTED]
  cmd/devsquad/main.go → internal/config/config.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/devsquad/main.go → internal/llm/anthropic/anthropic.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Approval gate flow (request, decide, resume)** — docs_10_prd_approval_gate, docs_14_backend_gate_node, docs_14_backend_approvalservice_decide, docs_15_api_approval_decide_endpoint, docs_02_crewai_langgraph_interrupt_resume [INFERRED 0.85]
- **Event ledger to realtime UI pipeline** — docs_15_api_event_schema, docs_15_api_transactional_event_commit, docs_15_api_ws_protocol, docs_13_frontend_ws_reconnect, docs_13_frontend_eventstore [INFERRED 0.85]
- **Docker Compose dev/test stack** — infra_docker_compose_postgres, infra_docker_compose_devsquad, infra_docker_compose_web, infra_docker_compose_test_postgres, github_workflows_ci_go_job [EXTRACTED 1.00]
- **Reconcile 기반 Stage 실행·커밋 흐름** — docs_19_go___________reconcile, docs_19_go___________attempt_fencing, docs_19_go___________commit_functions, docs_19_go___________emitter_bus, docs_19_go___________task_status_derivation, docs_19_go___________stage_state_machine [EXTRACTED 1.00]
- **팀원 정의 파일 세트** — docs_17_agent________persona_md, docs_17_agent________conventions_md, docs_17_agent________knowledge_dir, docs_17_agent________spec_md, docs_17_agent________pipeline_yaml, docs_17_agent________tool_profile [EXTRACTED 1.00]
- **Go 전환 문서 계보 (포팅→통합 설계→구현 검증→후속 기능)** — docs_18_go____doc, docs_19_go___________doc, docs_20_go______________doc, docs_21____________doc [INFERRED 0.85]
- **Designer -> Backend -> Frontend Contract Handoff** — examples_devsquad_templates__devsquad_agents_designer_conventions_design_document, examples_devsquad_templates__devsquad_agents_designer_conventions_components_json, examples_devsquad_templates__devsquad_agents_backend_conventions_api_spec_document, examples_devsquad_templates__devsquad_agents_backend_conventions_devsquad_openapi_block, examples_devsquad_templates__devsquad_agents_frontend_conventions_api_usage_document, examples_devsquad_templates__devsquad_agents_frontend_conventions_devsquad_api_usage_block [INFERRED 0.85]
- **Loading / Empty / Error Screen State Handling** — examples_devsquad_templates__devsquad_agents_designer_conventions_four_screen_states, examples_devsquad_templates__devsquad_agents_designer_knowledge_component_inventory_skeleton, examples_devsquad_templates__devsquad_agents_designer_knowledge_component_inventory_emptystate, examples_devsquad_templates__devsquad_agents_designer_knowledge_component_inventory_inlineerror [EXTRACTED 1.00]
- **Withdrawal API Call Layer (schema -> api -> query hook)** — examples_devsquad_templates__devsquad_agents_frontend_knowledge_frontend_conventions_withdrawal_schemas, examples_devsquad_templates__devsquad_agents_frontend_knowledge_frontend_conventions_requestwithdrawal, examples_devsquad_templates__devsquad_agents_frontend_knowledge_frontend_conventions_userequestwithdrawal, examples_devsquad_templates__devsquad_agents_frontend_knowledge_frontend_conventions_apiclient_post [EXTRACTED 1.00]
- **devsquad:* machine-readable YAML deliverable blocks** — examples_devsquad_templates__devsquad_agents_planner_conventions_devsquad_summary_block, examples_devsquad_templates__devsquad_agents_qa_conventions_devsquad_qa_block, examples_devsquad_templates__devsquad_agents_reviewer_conventions_devsquad_review_block [INFERRED 0.85]
- **Default DevSquad pipeline flow** — examples_devsquad_templates__devsquad_pipeline_stage_planning, examples_devsquad_templates__devsquad_pipeline_stage_design, examples_devsquad_templates__devsquad_pipeline_stage_backend, examples_devsquad_templates__devsquad_pipeline_stage_frontend, examples_devsquad_templates__devsquad_pipeline_stage_review, examples_devsquad_templates__devsquad_pipeline_stage_pr [EXTRACTED 1.00]
- **Agent definition = persona + conventions + knowledge** — examples_devsquad_templates__devsquad_agents_planner_persona_planner, examples_devsquad_templates__devsquad_agents_planner_conventions_planner_conventions, examples_devsquad_templates__devsquad_agents_planner_knowledge_readme_planner_knowledge_index, examples_devsquad_templates__devsquad_agents_reviewer_persona_reviewer, examples_devsquad_templates__devsquad_agents_reviewer_conventions_reviewer_conventions, examples_devsquad_templates__devsquad_agents_reviewer_knowledge_readme_reviewer_knowledge_index [INFERRED 0.85]

## Communities (47 total, 12 thin omitted)

### Community 0 - "Go 진입점·HTTP·WS"
Cohesion: 0.09
Nodes (12): ApprovalQuery, Decide, Empty, EventsQuery, ID, response, TaskApprovalQuery, TasksQuery (+4 more)

### Community 1 - "이벤트 Bus·통합 테스트"
Cohesion: 0.05
Nodes (47): EventPage, main(), run(), subscriber, Subscription, harness, runnerFunc, ProjectService (+39 more)

### Community 2 - "Go 포팅·로드맵 설계 문서"
Cohesion: 0.06
Nodes (47): ApprovalService.decide (낙관적 락, resume_pending) CP-6, 구현 로드맵 — Phase 1~3 작업 분해, EventDispatcher + Redis Stream consumer (CP-5), Fake LLM 모드, 모노레포 구성 (control-plane/agent-runtime/web/infra), Phase 2 팀 (4 agent + 병렬 + Discord), Phase 3 결과물 (GitHub PR + 상호 검수 + 예산), RunRecoveryJob (CP-8) (+39 more)

### Community 3 - "백엔드 팀원 템플릿"
Cohesion: 0.05
Nodes (50): Backend Conventions, docs/api/<task-slug>.md API Spec, Backend Definition of Done, devsquad:openapi Block, Testcontainers Integration Tests, API Guidelines, Idempotency-Key 24h Rule, Path Versioning /api/v1 (+42 more)

### Community 4 - "Task·Approval 앱 서비스"
Cohesion: 0.07
Nodes (28): ApprovalEntity, Coordinator, DecidedResponse, LastEventResponse, Page, PendingApprovalResponse, StageResponse, TaskResponse (+20 more)

### Community 5 - "기획자 팀원 템플릿"
Cohesion: 0.07
Nodes (45): Planner Definition of Done, devsquad:summary YAML Block, Planner Plan Format, Planner Conventions, Requirements Definition Document (docs/spec/<task-slug>.md), Planner Knowledge Index, PRD Checklist (prd-checklist.md), Commonly Missed Requirements (empty state, delete semantics, permission, concurrent edit, sort/paging) (+37 more)

### Community 6 - "Orchestrator·Stage 실행기"
Cohesion: 0.10
Nodes (22): AgentSpec, Definition, StageEntity, Derive(), Ready(), Terminal(), TestPipelineValidation(), Orchestrator (+14 more)

### Community 7 - "웹 에이전트 활동 패널"
Cohesion: 0.11
Nodes (27): @testing-library/react, AgentActivityPanel(), AgentCard(), colors, PendingApproval, Stage, StageStatus, Task (+19 more)

### Community 8 - "설정·샌드박스·워크스페이스"
Cohesion: 0.08
Nodes (24): Config, checkWriteConflicts(), literalRoot(), LoadDefinition(), Load(), TestExampleDefinitionsAndWorkspace(), recoveryLoop(), TestRecoveryUsesFixedDelayAndStopsOnCancellation() (+16 more)

### Community 9 - "웹 이벤트 타임라인"
Cohesion: 0.14
Nodes (20): @tanstack/react-virtual, vitest, EventRow(), summary(), EventTimeline(), HIDDEN_TYPES, EventType, TaskEvent (+12 more)

### Community 10 - "LLM 어댑터"
Cohesion: 0.09
Nodes (19): Adapter, UsagePayload, LLM, TestAnthropicCacheAndToolResults(), New(), marker(), BudgetGuard, ChatRequest (+11 more)

### Community 11 - "웹 Task 목록·생성 화면"
Cohesion: 0.14
Nodes (19): react, EXAMPLES, NewTaskPage(), FILTERS, TasksPage(), Button(), styles, Variant (+11 more)

### Community 12 - "웹 WS 연결"
Cohesion: 0.14
Nodes (3): WsConnection, make(), MockSocket

### Community 13 - "웹 API 클라이언트·쿼리"
Cohesion: 0.14
Nodes (17): DecidedResponse, EventPage, Project, TaskPage, DecideBody, fetchEvents(), fetchPendingApprovals(), fetchTask() (+9 more)

### Community 14 - "Store 엔티티 매핑"
Cohesion: 0.15
Nodes (7): ApprovalEntity, ProjectEntity, TaskEntity, decode(), decodeList(), Tx, Reader

### Community 16 - "웹 tsconfig"
Cohesion: 0.11
Nodes (18): compilerOptions, allowJs, esModuleInterop, incremental, isolatedModules, jsx, lib, module (+10 more)

### Community 17 - "웹 런타임 의존성"
Cohesion: 0.12
Nodes (16): autoprefixer, jsdom, postcss, react-dom, react-markdown, remark-gfm, tailwindcss, @tailwindcss/postcss (+8 more)

### Community 18 - "sqlc 파라미터 타입"
Cohesion: 0.12
Nodes (12): ClaimStageParams, CreateApprovalParams, CreateDeliverableParams, CreateStageParams, CreateTaskParams, CreateUsageParams, DecideApprovalParams, ListApprovalsParams (+4 more)

### Community 19 - "웹 승인 패널·API 스키마"
Cohesion: 0.18
Nodes (12): zod, ApprovalPanel(), MarkdownView(), api, ApiError, request(), Schema, USER_HEADER (+4 more)

### Community 20 - "웹 Task 상세 화면"
Cohesion: 0.27
Nodes (11): TaskDetailPage(), StatusBadge(), PipelineBar(), useInitialEvents(), useTaskAction(), ConnectionStatus, ConnectionStore, useConnectionStore (+3 more)

### Community 21 - "웹 개발 의존성"
Cohesion: 0.14
Nodes (14): devDependencies, autoprefixer, jsdom, @playwright/test, postcss, tailwindcss, @tailwindcss/postcss, @testing-library/react (+6 more)

### Community 22 - "README·기여 규칙"
Cohesion: 0.19
Nodes (11): Contribution Rules (branches, Conventional Commits, scopes), DB schema (project, task, stage, approval, deliverable, task_event, usage_record), EventDispatcher, Public REST API /api/v1, CI go job, test postgres service (port 15432), DevSquad (approval-based AI dev team tool), DEVSQUAD_FAKE_LLM mode (+3 more)

### Community 23 - "sqlc 모델"
Cohesion: 0.26
Nodes (9): Approval, CreateEventParams, CreateProjectParams, Deliverable, Project, Stage, Task, TaskEvent (+1 more)

### Community 24 - "웹 앱 레이아웃"
Cohesion: 0.20
Nodes (6): next, @tanstack/react-query, metadata, RootLayout(), Providers(), nextConfig

### Community 25 - "PRD 승인 게이트 개념"
Cohesion: 0.22
Nodes (7): Approval (approve/reject/edit), Plan/Deliverable approval gate, Deliverable, DevSquad PRD, plan_gate / deliverable_gate node, Stage subgraph (plan, gate, execute, gate, commit_result), Discord slash commands and approval cards (future)

### Community 27 - "웹 package 의존성"
Cohesion: 0.20
Nodes (10): dependencies, next, react, react-dom, react-markdown, remark-gfm, @tanstack/react-query, @tanstack/react-virtual (+2 more)

### Community 28 - "CrewAI·LangGraph 기술 정리"
Cohesion: 0.22
Nodes (6): CrewAI, CrewAI Flows, LangGraph, Multi-agent patterns (Supervisor, Handoffs, Router, Skills, Custom workflow), Agent Runtime (Python FastAPI + LangGraph, initial design), Control Plane (Spring Boot, initial design)

### Community 29 - "팀원 정의·파이프라인 개념"
Cohesion: 0.25
Nodes (8): tool_profile (docs-writer/code-writer/reader/publisher) + write_paths, User-defined team members (.devsquad/agents), .devsquad context directory (spec.md, agents/), pipeline.yaml Stage DAG, Team member settings screen, System Prompt assembly (persona, conventions, spec, knowledge), Tool sandbox (workspace path, secret file deny), pipeline.yaml schema

### Community 30 - "Compose 서비스·CI"
Cohesion: 0.29
Nodes (8): AgentActivityPanel, eventStore (Zustand), CI web job, devsquad service (compose), postgres service (compose), web service (compose), CONTROL_PLANE_URL build-time proxy target, Next.js web app

### Community 31 - "웹 npm 스크립트"
Cohesion: 0.25
Nodes (8): scripts, build, dev, e2e, lint, start, test, typecheck

### Community 32 - "이벤트 envelope·seq"
Cohesion: 0.33
Nodes (5): Event envelope (event_id, task_id, seq, type, payload), WS reconnect + seq gap fill, zod WS event schema (unknown-tolerant), Event schema (task_event / WS shared), WebSocket /ws protocol (subscribe from_seq)

### Community 33 - "SCM Publisher 자리"
Cohesion: 0.48
Nodes (4): Draft, Noop, Publisher, Result

### Community 34 - "승인 멱등성·상태 기계"
Cohesion: 0.33
Nodes (4): Task state machine, ApprovalService.decide (optimistic lock), POST /approvals/{id}/decide, Error codes (APPROVAL_ALREADY_DECIDED, INVALID_TRANSITION...)

### Community 35 - "재시작 복구 개념"
Cohesion: 0.50
Nodes (3): Process restart recovery, RunRecoveryJob, Internal Reconcile execution

### Community 36 - "디자인 화면 원칙"
Cohesion: 0.50
Nodes (3): Component inventory (PipelineBar, AgentCard, EventTimeline, ApprovalPanel...), Design tokens (role/status colors, dark theme), Task detail 3-zone screen

## Ambiguous Edges - Review These
- `sqlc codegen config (pgx/v5)` → `DB schema (project, task, stage, approval, deliverable, task_event, usage_record)`  [AMBIGUOUS]
  docs/14-Backend-설계.md · relation: conceptually_related_to
- `Cursor Pagination (no offset)` → `TanStack Query Server State`  [AMBIGUOUS]
  examples/devsquad-templates/.devsquad/agents/backend/knowledge/api-guidelines.md · relation: conceptually_related_to

## Knowledge Gaps
- **137 isolated node(s):** `github.com/jinho-yoo-jack/devsquad`, `Empty`, `ID`, `TasksQuery`, `ApprovalQuery` (+132 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 220 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **12 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `sqlc codegen config (pgx/v5)` and `DB schema (project, task, stage, approval, deliverable, task_event, usage_record)`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `Orchestrator` connect `Orchestrator·Stage 실행기` to `Go 진입점·HTTP·WS`, `이벤트 Bus·통합 테스트`, `Task·Approval 앱 서비스`, `설정·샌드박스·워크스페이스`, `sqlc 쿼리 메서드`?**
  _High betweenness centrality (0.011) - this node is a cross-community bridge._
- **What connects `github.com/jinho-yoo-jack/devsquad`, `Empty`, `ID` to the rest of the system?**
  _137 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Go 진입점·HTTP·WS` be split into smaller, more focused modules?**
  _Cohesion score 0.09407216494845361 - nodes in this community are weakly interconnected._
- **What is the exact relationship between `Cursor Pagination (no offset)` and `TanStack Query Server State`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `vitest` connect `웹 이벤트 타임라인` to `Vitest 설정`, `웹 에이전트 활동 패널`, `웹 WS 연결`, `웹 API 클라이언트·쿼리`, `웹 런타임 의존성`?**
  _High betweenness centrality (0.008) - this node is a cross-community bridge._
- **Should `이벤트 Bus·통합 테스트` be split into smaller, more focused modules?**
  _Cohesion score 0.05213089802130898 - nodes in this community are weakly interconnected._