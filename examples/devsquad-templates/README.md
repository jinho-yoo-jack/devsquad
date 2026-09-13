# DevSquad 팀원 템플릿

`.devsquad/` 폴더를 자기 저장소 루트에 복사하면 팀이 생긴다. 규격과 작성 원칙은 `17-Agent-정의-가이드.md`를 본다.

```
.devsquad/
├── spec.md                 프로젝트 공통 컨텍스트 — TODO 를 채운다
├── pipeline.yaml           조직도 — 팀원을 빼거나 추가한다
└── agents/
    ├── planner/            기획자        docs-writer   → docs/spec/
    ├── designer/           디자이너      docs-writer   → docs/design/ (+ components.json)
    ├── backend/            백엔드 개발자  code-writer   → docs/api/ + server/ 커밋
    ├── frontend/           프론트엔드    code-writer   → web/ 커밋 + docs/api-usage/
    ├── reviewer/           검수자        reader        → docs/review/
    └── qa/                 QA (확장 예시, 기본 파이프라인 미포함)
```

## 시작하기
1. `spec.md`의 `<!-- TODO -->`를 채운다. 특히 §3 저장소 구조와 §5 용어집.
2. 스택이 Spring + Next.js가 아니면 각 팀원의 `persona.md` `write_paths`·`test_command`와 `conventions.md` §3을 고친다.
3. `backend/knowledge/db-schema.md`를 실제 스키마로, `designer/knowledge/component-inventory.md`를 실제 컴포넌트로 바꾼다.
4. 팀원이 필요 없으면 `pipeline.yaml`에서 Stage를 지우고 `depends_on`을 이어 준다. 폴더는 남겨 둬도 된다.
5. 첫 Task는 작게. "설정 페이지에 다크 모드 토글 추가" 정도로 파이프라인이 끝까지 도는지 본다.

## 팀원 추가하기
`agents/qa/`를 복사해 이름을 바꾸고, `persona.md`의 `tool_profile`·`write_paths`를 정한 뒤 `pipeline.yaml`에 Stage를 추가한다. `conventions.md`의 §1(입력)과 §4(결과물 형식)를 먼저 쓰고, 앞뒤 팀원의 §1/§4와 파일 경로가 맞는지 확인한다.

## 각 팀원이 남기는 것 → 사용자에게 보이는 곳
| 팀원 | 파일 | 웹 뷰어 | Discord | GitHub |
|---|---|---|---|---|
| planner | `docs/spec/<slug>.md` | Markdown | 요약 카드 | PR에 포함 |
| designer | `docs/design/<slug>.md`, `.components.json` | Markdown + JSON | 요약 카드 | PR에 포함 |
| backend | `docs/api/<slug>.md`, `server/**` | Markdown + diff | 변경 요약 | 브랜치 → PR |
| frontend | `web/**`, `docs/api-usage/<slug>.md` | diff + Markdown | 변경 요약 | 브랜치 → PR |
| reviewer | `docs/review/<slug>.md` | Markdown | 판정 + 건수 | PR 본문 요약 |
| qa | `web/e2e/<slug>.spec.ts`, `docs/qa/<slug>.md` | diff + Markdown | 통과/실패 수 | 브랜치 → PR |
