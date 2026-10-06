# 개발 규칙

- 브랜치: `master` 보호. 작업은 `feat/<area>-<slug>`, `fix/…`, `docs/…` 브랜치에서 PR.
- 커밋: Conventional Commits. scope 는 `api` / `orchestrator` / `llm` / `store` / `web` / `infra` / `docs` / `templates`.
  예) `feat(orchestrator): recover interrupted stages`
- 로드맵 작업 ID 를 PR 제목이나 본문에 적는다. 예) `[G4] 단계 재시작 복구`
- 계약(15-API-이벤트-명세)을 바꾸는 PR 은 문서와 코드를 같은 PR 에서 고친다.
- 시크릿은 절대 커밋하지 않는다. `infra/.env.example` 만 갱신한다.

- Go 변경은 `make test build`, DB·오케스트레이터 변경은 `make test-integration`을 실행한다.
- SQL 변경은 `make generate`로 sqlc 코드를 재생성한다. 트랜잭션 경계는 app과 orchestrator에 둔다.
