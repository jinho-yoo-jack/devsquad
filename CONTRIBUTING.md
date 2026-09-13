# 개발 규칙

- 브랜치: `main` 보호. 작업은 `feat/<area>-<slug>`, `fix/…`, `docs/…` 브랜치에서 PR.
- 커밋: Conventional Commits. scope 는 `cp`(control-plane) / `ar`(agent-runtime) / `web` / `infra` / `docs` / `templates`.
  예) `feat(ar): add plan_gate node with interrupt()`
- 로드맵 작업 ID 를 PR 제목이나 본문에 적는다. 예) `[AR-5] stage subgraph`
- 계약(15-API-이벤트-명세)을 바꾸는 PR 은 문서와 코드를 같은 PR 에서 고친다.
- 시크릿은 절대 커밋하지 않는다. `infra/.env.example` 만 갱신한다.
