# 🧩 DevSquad

사용자가 정의한 AI 팀원들이 **계획 → 사람의 승인 → 실행 → 결과 승인** 순서로 작업하는 개발 도구입니다.

백엔드는 **Go 1.27.1 단일 서비스 + PostgreSQL**, 웹은 **Next.js 15 / React 19**입니다. `.devsquad/`의 팀원과 DAG 정의를 읽고, Task·Stage·Approval 행을 실행 체크포인트로 사용합니다.

```mermaid
flowchart LR
    Web[Next.js] -->|REST / WebSocket| Go[devsquad · Huma]
    Go --> PG[(PostgreSQL)]
    Go --> Runner[goroutine Stage Runner]
    Runner --> LLM[Anthropic / OpenAI / Ollama / Fake]
    Runner --> Workspace[Task별 Git 작업 디렉토리]
```

## 구현 범위

- 기존 `/api/v1` REST 및 `/ws` 계약, OpenAPI `/openapi.json`, API 문서 `/docs`
- 단계별 의존성 실행, 계획·결과 승인, 수정·반려 상한, 일시정지·재개·취소
- 프로세스 재시작 복구, 실행 시도 번호를 통한 중복 결과 차단, 단계 쓰기 경로별 파일 복원
- 트랜잭션 이벤트 저장, 커밋 후 내부 Bus 전송, WebSocket 재생·느린 구독자 분리
- 공식 LLM SDK, Anthropic 프롬프트 캐시, ReAct 파일 도구, 사용량 기록, `/metrics`
- Task 상세의 에이전트별 현재 작업·도구 결과·대기 이유 표시, 실시간 갱신과 단계별 타임라인 필터
- Task별 토큰 예산: 모델 호출 전 검사, 초과 시 자동 일시정지(`run.paused{reason:"budget"}`), 증액 후 재개
- 단일 사용자 로그인: 관리자 비밀번호로 장기 JWT(HttpOnly 쿠키 또는 Bearer)를 발급하고 API·WebSocket에 적용
- GitHub PR: `local_path`가 없는 프로젝트는 GitHub에서 clone하고, 내장 `publisher` 단계가 승인된 결과를 `devsquad/<task-id>/…` 브랜치로 push한 뒤 PR을 열거나 재사용

Discord는 후속 범위입니다. 현재는 인스턴스 1개를 실행하며, 같은 DB 스키마에 두 번째 서비스를 띄우면 시작을 거부합니다.

### GitHub PR

`DEVSQUAD_GITHUB_TOKEN`에 Contents·Pull requests 쓰기 권한이 있는 토큰을 설정하고, 프로젝트를 `local_path` 없이 `github_owner`·`github_repo`·`default_branch`로 등록합니다. Task를 만들면 기본 브랜치를 clone하며 토큰은 `.git/config`에 저장하지 않습니다. `pipeline.yaml`의 `publisher` 단계는 모델을 호출하지 않고 다음을 합니다.

- `policy.pr.mode: single`(기본): 승인된 전체 변경을 `devsquad/<task-id>` 브랜치 하나와 PR 하나로
- `policy.pr.mode: per-role`: 단계마다 소유 경로의 변경만 `devsquad/<task-id>/<role>` 브랜치와 PR로
- PR 본문: Task 요약, 산출물 링크, 단계별 요약, 승인 이력. 같은 브랜치의 열린 PR은 갱신해 재사용
- push·PR 실패 시 단계는 `blocked`(`stage.blocked{reason:"publish_failed"}`)가 되고, 권한을 고친 뒤 재개하면 다시 시도

`local_path` 프로젝트는 지금처럼 복사해 실행하고 `publisher`는 PR 본문 초안만 남깁니다.

## 실행

필요 도구: Go 1.27.1, Git, Docker Compose. 웹 개발에는 Node 22 이상이 필요합니다.

```bash
cp infra/.env.example infra/.env
# infra/.env에서 DEVSQUAD_ADMIN_PASSWORD와 DEVSQUAD_JWT_SECRET(32자 이상, 예: openssl rand -hex 32)을 설정합니다.
# DEVSQUAD_FAKE_LLM=true로 설정하면 모델 호출 없이 실행합니다.
docker compose -f infra/docker-compose.yml --profile full up --build
```

- 웹: http://localhost:3000 (관리자 비밀번호로 로그인)
- 서비스: http://localhost:8080/api/v1/health
- API 문서: http://localhost:8080/docs

예제 프로젝트의 `local_path`는 `/projects/templates`입니다. 다른 저장소는 `devsquad` 서비스에 읽기 전용으로 마운트하고 컨테이너 내부 경로로 등록합니다. 기본 이미지에는 Git과 Bash가 있습니다. 실제 `run_tests`에 필요한 Go·Node·Java 등은 대상 프로젝트에 맞춰 이미지에 추가합니다.

로컬 개발:

```bash
docker compose -f infra/docker-compose.yml up -d --wait postgres
cp infra/.env.example .env
# .env에서 DEVSQUAD_FAKE_LLM=true 또는 모델 키, 그리고 로그인 비밀번호·JWT 시크릿을 설정합니다.
# 로컬에서 인증 없이 쓰려면 DEVSQUAD_AUTH_DISABLED=true를 명시합니다.
make run

# 별도 터미널
cd web
npm ci
npm run dev
```

서비스는 현재 디렉토리의 `.env`를 자동으로 읽으며 이미 설정된 환경 변수가 우선합니다. 작업 디렉토리는 원본 저장소 밖에 둡니다. PostgreSQL의 기본 포트가 사용 중이면 `DEVSQUAD_POSTGRES_PORT`와 로컬 `DEVSQUAD_DB_URL` 포트를 함께 변경합니다.

## 검증

```bash
make test build
make test-integration
make generate  # sqlc v1.31.1로 SQL 코드 재생성
cd web && npm run typecheck && npm test -- --run && npm run build
```

통합 테스트는 별도 PostgreSQL 포트 `15432`와 테스트별 스키마를 사용합니다. 일반 `make test`에서는 DB 환경 변수가 없으면 DB 테스트를 건너뛰며, `make test-integration`에서는 반드시 실행합니다.

```bash
docker compose -p devsquad-go-test -f infra/docker-compose.test.yml down -v
```

실제 OpenAI 파이프라인 테스트는 명시적으로 켤 때만 모델을 호출합니다.

```bash
DEVSQUAD_TEST_DB_URL=postgres://devsquad:devsquad@localhost:15432/devsquad \
DEVSQUAD_REAL_LLM_TEST=true go test -v -run TestLiveOpenAIPipeline ./internal/integration
```

셸에 `OPENAI_API_KEY`가 필요합니다. 기본 테스트 모델은 `gpt-4o-mini`, 변경은 `DEVSQUAD_REAL_MODEL`로 지정합니다.

## 기존 설치 전환

새 DB에 goose 마이그레이션을 적용합니다. 이전 Flyway/LangGraph/Temporal 데이터의 자동 변환은 제공하지 않습니다. 기존 데이터를 보존해야 하면 새 DB와 새 workspace로 실행하고 기존 인스턴스는 별도로 보관합니다. `make resetdb`는 **이 Compose 프로젝트의 DB와 workspace 볼륨을 삭제**하는 개발용 명령입니다.

## 코드와 문서

| 위치 | 역할 |
|---|---|
| `cmd/devsquad` | 설정·마이그레이션·서비스 조립 |
| `internal/httpapi`, `app`, `domain`, `store` | API, 트랜잭션, 상태 기계, sqlc/pgx/goose |
| `internal/orchestrator`, `stage` | Reconcile·복구, 계획·ReAct 실행 |
| `internal/llm`, `tools`, `workspace` | SDK, 파일 도구, Git 기준점 |
| `internal/event`, `ws` | 이벤트 원장과 실시간 구독 |
| `internal/integration` | 실제 PostgreSQL 통합·재시작·모델 테스트 |
| `web` | Next.js 작업·승인 화면, 에이전트 작업 현황 |

- [19 · 통합 서비스 설계](docs/19-Go-통합-서비스-설계.md)
- [20 · 구현·검증 기록](docs/20-Go-통합-서비스-구현-검증.md)
- [API·이벤트 계약](docs/15-API-이벤트-명세.md)
- [팀원 정의 가이드](docs/17-Agent-정의-가이드.md)
- [PRD](docs/10-PRD.md), [로드맵](docs/16-구현-로드맵.md)

[MIT](LICENSE) · [기여 규칙](CONTRIBUTING.md)
