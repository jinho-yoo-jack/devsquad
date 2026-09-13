---
title: PRD — DevSquad (가칭): 승인 기반 AI 개발팀 서비스
type: prd
project: 나만의 Agent 만들기
status: draft v0.1
created: 2026-09-11
tags: [prd, ai-agent, multi-agent, human-in-the-loop]
---

# PRD — DevSquad (가칭)

> [!tip] 핵심 Takeaway
> DevSquad는 기획·디자인·Frontend·Backend 네 명의 AI 팀원이 사용자의 한 줄 명령을 받아 **각자 할 일을 계획하고, 사람의 승인을 받은 뒤에만 실행**하는 개발팀 서비스다. 승인은 Discord 또는 웹에서 하고, 결과는 GitHub PR로 나오며, 팀원들이 일하는 과정은 웹 UI에서 실시간으로 본다. MVP는 세 Phase로 나누며 Phase 1은 "기획 agent 1명 + 승인 게이트 + 웹 관찰"만으로 끝까지 도는 것을 목표로 한다.

← [[00-프로젝트-개요]] · 다음: [[11-시스템-아키텍처]]

## 1. 배경과 문제

AI 코딩 도구는 "이거 해줘"라고 시키면 바로 코드를 쏟아낸다. 그러나 실제 개발은 기획이 먼저 정해지고, 화면이 그려지고, API 계약이 합의된 뒤에 구현이 시작된다. 지금의 도구는 이 순서를 건너뛰기 때문에 결과물이 요구와 어긋나거나, 중간에 사람이 개입할 지점이 없어 한 번 잘못 가면 전부 다시 해야 한다.

DevSquad가 풀려는 문제는 두 가지다. 첫째, **역할 분리**: 기획·디자인·FE·BE가 각자의 컨텍스트(`CLAUDE.md`, `spec.md`, 도메인 지식)를 갖고 자기 관점에서 일한다. 둘째, **승인 게이트**: 각 agent는 "무엇을 할지"를 먼저 계획으로 제출하고, 사람이 승인해야 실행한다. 실행 결과도 다시 승인을 거쳐 다음 agent로 넘어간다. 사람은 실행자가 아니라 **리뷰어**가 된다.

## 2. 목표와 비목표

### 목표

- 사용자가 한 줄 명령(예: "회원 탈퇴 기능 추가")을 내리면 기획 → 디자인 → FE/BE → 리뷰 → PR까지의 파이프라인이 자동으로 돈다.
- 모든 단계에 **계획 승인**과 **결과 승인** 두 개의 게이트가 있다. 반려 시 agent가 피드백을 반영해 다시 제출한다.
- 승인은 Discord와 웹 UI 어디서 해도 같은 효과를 낸다.
- 웹 UI에서 각 agent의 현재 상태, 사고 과정(요약), 도구 호출, 산출물을 실시간으로 볼 수 있다.
- 실행이 며칠 멈춰 있어도(승인 대기) 서버 재시작 후 그 자리에서 이어진다.
- 각 agent의 페르소나·컨벤션·도메인 지식은 **코드가 아닌 파일**로 관리되어, 프로젝트마다 갈아 끼울 수 있다.
- **팀 구성은 사용자가 정한다.** 기획·디자인·FE·BE는 서비스가 제공하는 예시 팀원(템플릿)일 뿐이며, 사용자는 팀원을 빼거나, 이름을 바꾸거나, 새 역할(QA, 인프라, 테크라이터 등)을 추가해 자기 팀을 구성한다. 서비스는 팀원을 정의하는 **규격**과 **템플릿 라이브러리**를 제공한다(17-Agent-정의-가이드).

### 비목표 (MVP 범위 밖)

- 일반 대화형 챗봇 기능.
- 완전 자율 실행(승인 없이 끝까지). 옵션으로도 제공하지 않는다.
- 다중 조직·과금·SSO 같은 SaaS 운영 기능.
- 실제 배포(CD)까지의 자동화. PR 생성에서 멈춘다.
- Slack, Jira 등 Discord/GitHub 이외 채널.

## 3. 사용자와 시나리오

주 사용자는 **1인 개발자 또는 소규모 팀의 리드**다. 자기 프로젝트에 기능을 추가하고 싶지만 기획서·화면·API·코드를 혼자 다 쓰기엔 시간이 없고, 그렇다고 AI에게 통째로 맡기기엔 불안한 사람이다.

대표 시나리오는 다음과 같다.

Jin Ho가 Discord `#devsquad` 채널에 `/task 회원 탈퇴 기능 추가. 탈퇴 후 30일 유예, 유예 기간 내 복구 가능`이라고 입력한다. 몇 초 뒤 기획 agent가 "제가 할 일"로 요구사항 정의서, 유스케이스 3개, 화면 목록, 비기능 요구 초안을 만들겠다는 **계획**을 Discord 카드로 올린다. Jin Ho가 ✅를 누른다. 기획 agent가 작업을 수행하고 `spec/withdrawal.md`를 결과로 제출한다. Jin Ho가 웹 UI에서 문서를 읽고 "유예 기간 중 로그인 시 복구 안내 모달 필요"라고 코멘트를 남기며 반려한다. 기획 agent가 수정 후 재제출하고 승인된다. 이어서 디자인 agent가 화면 설계 계획을 올리고… FE와 BE agent는 승인된 디자인·API 계약을 바탕으로 **병렬로** 계획을 올린다. 둘의 결과가 나오면 상호 검수(FE가 BE의 API 명세를 대조)가 자동으로 돌고, 통과하면 GitHub에 PR 두 개가 열리며 Discord에 링크가 올라온다. 이 모든 과정에서 웹 UI에는 네 agent의 타임라인이 흐르고, 어느 agent가 지금 무엇을 하는지 카드로 보인다.

## 4. 핵심 개념 정의

| 용어 | 정의 |
|---|---|
| Project | agent 팀이 작업하는 대상 코드베이스와 그 컨텍스트 파일 세트. GitHub repo 1개에 대응 |
| Agent | 역할(기획/디자인/FE/BE)을 가진 팀원. 페르소나·컨벤션·지식 파일로 정의 |
| Task | 사용자의 한 줄 명령으로 생성되는 최상위 작업 단위. 하나의 LangGraph thread에 대응 |
| Stage | Task 안에서 한 agent가 담당하는 구간. 기획 → 디자인 → (FE ∥ BE) → 리뷰 → PR |
| Plan | agent가 Stage 시작 시 제출하는 "내가 할 일" 목록. 승인 대상 1 |
| Deliverable | agent가 Stage 종료 시 제출하는 산출물(문서, 코드 diff, JSON). 승인 대상 2 |
| Approval | Plan 또는 Deliverable에 대한 사람의 결정. approve / reject(피드백 포함) / edit(수정 후 승인) |
| Event | agent 실행 중 발생하는 관찰 가능한 사건. 웹 UI 타임라인의 원천 |

## 5. 기능 요구사항

### 5.1 Task 생성과 파이프라인

- FR-01 사용자는 Discord slash command 또는 웹 UI에서 자연어 명령으로 Task를 생성한다.
- FR-02 Task는 Project의 `pipeline.yaml`이 정한 Stage DAG를 따른다. 기본 템플릿은 기획 → 디자인 → FE ∥ BE → 검수 → PR이지만, 사용자가 Stage를 빼거나 추가할 수 있다. Phase 1은 템플릿 중 planning Stage 하나만 있는 pipeline으로 시작한다.
- FR-03 각 Stage는 Plan 제출 → Plan 승인 → 실행 → Deliverable 제출 → Deliverable 승인의 순서를 따른다.
- FR-04 FE와 BE Stage는 병렬 실행되며, 둘의 Deliverable이 모두 승인되어야 상호 검수 Stage로 넘어간다.
- FR-05 사용자는 Task를 언제든 일시정지·취소할 수 있다.

### 5.2 승인

- FR-10 Plan과 Deliverable은 Discord 메시지(버튼 포함)와 웹 UI 승인 카드로 동시에 제시된다.
- FR-11 승인 결정은 approve, reject(피드백 필수), edit(내용을 직접 수정한 뒤 승인) 세 가지다.
- FR-12 어느 채널에서 결정해도 다른 채널의 카드는 결정 상태로 갱신된다.
- FR-13 reject 시 agent는 피드백을 반영해 재제출한다. 재제출 횟수 상한(기본 3)을 넘으면 Task가 `blocked` 상태로 멈추고 사용자에게 알린다.
- FR-14 승인 대기는 시간 제한이 없다. 서버가 재시작되어도 대기 상태와 컨텍스트가 유지된다.

### 5.3 Agent와 컨텍스트

- FR-20 각 agent는 Project 저장소의 `.devsquad/agents/<agent-name>/` 아래 `persona.md`(역할·목표·배경 + 프론트매터 `tool_profile`, `write_paths`), `conventions.md`(입력·Plan 형식·작업 규칙·결과물 형식·완료 기준·금지 사항의 6절), `knowledge/`(도메인 자료)를 읽어 System Prompt를 구성한다. 규격은 17-Agent-정의-가이드 §2.
- FR-21 Project 공통 `.devsquad/spec.md`는 모든 agent에 주입된다.
- FR-22 agent는 자기 Stage의 선행 Deliverable(승인된 것만)을 컨텍스트로 받는다. 어느 선행 산출물을 읽는지는 그 agent의 `conventions.md` §1이 정한다.
- FR-23 agent가 쓸 수 있는 도구는 **이름이 아니라 `tool_profile`** 로 결정된다. 프로필은 `docs-writer`, `code-writer`, `reader`, `publisher` 넷이며 쓰기 경로는 `write_paths`로 좁힌다. 사용자가 팀원 이름을 자유롭게 지어도 보안 경계는 프로필이 지킨다.
- FR-24 위험 도구(브랜치 push, PR 생성)는 Deliverable 승인 이후에만 실행된다.
- FR-25 **사용자 정의 팀원**: 사용자는 `.devsquad/agents/`에 폴더를 추가하고 `pipeline.yaml`에 Stage를 넣는 것만으로 팀원을 추가·제거·교체할 수 있다. 서비스는 Task 생성 시 폴더·필수 파일·`tool_profile` 유효성·`write_paths` 충돌(두 팀원이 같은 경로에 쓰기)을 검증한다.
- FR-26 **템플릿 라이브러리**: 서비스는 완성된 예시 팀원(MVP: planner, designer, backend, frontend, reviewer + 확장 예시 qa)과 `spec.md`, `pipeline.yaml` 예시를 제공한다. `POST /projects/{id}/bootstrap`과 웹 "팀원 추가"가 선택한 템플릿을 사용자 repo에 커밋한다.
- FR-27 결과물 형식은 각 팀원의 `conventions.md` §4가 정의하며, 다음 팀원이 기계적으로 읽을 수 있게 문서 끝에 `devsquad:*` YAML 블록(summary / openapi / api-usage / review)을 둔다. 서비스는 이 블록을 파싱해 검수 판정(`devsquad:review.verdict`)과 UI 요약에 사용한다.

### 5.4 Discord 연동

- FR-30 `/task <명령>`, `/status [task_id]`, `/pause`, `/resume`, `/cancel` slash command.
- FR-31 Task마다 Discord thread를 생성하고 그 안에 Plan/Deliverable 카드, 진행 알림, PR 링크를 올린다.
- FR-32 카드의 버튼(✅ 승인 / ❌ 반려 / ✏️ 수정)으로 승인한다. 반려는 모달로 피드백을 받는다.

### 5.5 GitHub 연동

- FR-40 Project는 GitHub repo 하나와 연결되며 GitHub App 설치로 권한을 얻는다.
- FR-41 FE/BE agent는 `devsquad/<task-id>/<role>` 브랜치에 커밋한다.
- FR-42 상호 검수 통과 후 PR agent가 PR을 생성한다. PR 본문에는 Task 요약, 승인된 spec 링크, 각 Stage의 Deliverable 요약, 승인 이력이 포함된다.
- FR-43 PR 리뷰 코멘트가 달리면 Discord thread와 웹 UI에 알림한다(MVP에서는 자동 반영은 하지 않음).

### 5.6 웹 UI (관찰·승인)

- FR-50 Task 목록: 상태별 필터, 진행률, 현재 Stage, 대기 중인 승인 수.
- FR-51 Task 상세: Stage 파이프라인 시각화(진행/대기/완료/반려), 각 agent 카드(상태, 현재 사고 요약, 마지막 도구 호출), 이벤트 타임라인(실시간 스트림), 산출물 뷰어(Markdown, diff, JSON), 승인 패널.
- FR-52 승인 패널에서 approve / reject / edit이 가능하며 edit은 Markdown 편집기를 제공한다.
- FR-53 Agent 설정 화면: Project별 팀원 목록을 보고, 템플릿에서 팀원을 추가하거나 빈 팀원을 만들고, 페르소나·컨벤션·지식 파일을 읽고 편집(GitHub에 커밋)하며, `pipeline.yaml`에서의 위치(depends_on)를 확인한다.
- FR-55 산출물 공유: 모든 Deliverable은 (a) 웹 산출물 뷰어(원문), (b) Discord thread 카드(요약 + 링크), (c) 사용자 repo의 파일·브랜치·PR, (d) `GET /deliverables/{id}` API 네 경로로 사용자에게 전달된다. 서비스 DB에만 존재하는 결과물은 없다.
- FR-54 실시간 갱신은 WebSocket(또는 SSE)으로, 새로고침 없이 반영된다.

## 6. 비기능 요구사항

- NFR-01 내구성: 실행 상태는 PostgreSQL에 체크포인트로 저장되며 어느 시점에 프로세스가 죽어도 마지막 super-step에서 재개된다.
- NFR-02 멱등성: Discord 버튼 중복 클릭, 웹훅 재전송에도 승인이 두 번 적용되지 않는다(approval_id 기준).
- NFR-03 관측: 모든 agent 이벤트는 구조화 로그로 남고, LLM 호출별 토큰·비용이 Task 단위로 집계된다.
- NFR-04 비용 상한: Task별 토큰 예산(기본값 설정 가능)을 넘으면 자동 일시정지.
- NFR-05 보안: GitHub App private key, Discord bot token, LLM API key는 환경 변수/Secret Manager로만 주입한다. agent가 읽는 저장소 파일에서 `.env`, 키 파일 패턴은 도구 계층에서 차단한다.
- NFR-06 응답성: 승인 후 agent가 실행을 시작하기까지 5초 이내, 이벤트가 웹 UI에 반영되기까지 1초 이내(P95).
- NFR-07 Python agent 서버와 Spring 서버는 독립 배포·독립 스케일이 가능해야 한다.

## 7. 성공 지표 (MVP)

- Phase 1: 명령 → 기획 Plan → 승인 → Deliverable → 승인까지 웹 UI에서 끝까지 도는 것. 서버 재시작 후 승인 대기 재개 성공.
- Phase 2: 4 agent 파이프라인이 Discord 승인만으로 끝까지 도는 것. FE/BE 병렬 실행 확인.
- Phase 3: 실제 repo에 PR이 생성되고, 상호 검수에서 API 불일치를 1건 이상 잡아내는 것.

## 8. Phase 범위

| Phase | 범위 | 제외 |
|---|---|---|
| 1 | 기획 agent 1개, Plan/Deliverable 승인 게이트, 웹 UI(Task 상세·타임라인·승인 패널), Postgres 체크포인트, 서버 재시작 재개 | Discord, GitHub, 다른 agent |
| 2 | 디자인·FE·BE agent 추가, FE∥BE 병렬, Discord slash command·카드·thread, 채널 간 승인 동기화, Agent 설정 화면 | GitHub 쓰기 |
| 3 | GitHub App 연동, 브랜치 커밋, 상호 검수 Stage, PR 생성, PR 코멘트 알림, 토큰 예산 | 자동 배포 |

## 9. 열린 질문

- 디자인 agent의 산출물 형식: Markdown 화면 설계서만인가, HTML 목업까지인가 (MVP는 Markdown + 컴포넌트 트리 JSON으로 제한 제안).
- edit 승인 시 사람이 수정한 Deliverable을 그대로 다음 Stage로 넘길지, agent가 한 번 더 정리하게 할지.
- 상호 검수에서 불일치가 나오면 어느 agent가 고치는가 (제안: BE 계약을 기준으로 FE가 수정).
