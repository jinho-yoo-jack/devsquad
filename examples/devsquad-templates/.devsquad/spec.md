# 프로젝트 공통 컨텍스트 (spec.md)

> 이 파일은 모든 팀원 agent에게 주입된다. 2~3페이지를 넘기지 말고, 상세는 각 팀원의 `knowledge/`로 내린다.
> `<!-- TODO -->` 표시를 자기 프로젝트 내용으로 바꿔라.

## 1. 제품

<!-- TODO: 한 단락. 무엇을 만드는 서비스인가, 누가 쓰는가, 핵심 가치는 무엇인가 -->
예시: "Bookmarkly는 개인용 북마크 관리 웹 서비스다. 사용자는 URL을 저장하고 태그로 분류하며, 브라우저 확장으로 한 번에 저장한다. 핵심 가치는 '저장 3초, 검색 1초'다."

## 2. 사용자

<!-- TODO: 주 사용자 1~2 유형과 그들이 중요하게 여기는 것 -->

## 3. 기술 스택과 저장소 구조

```
repo/
├── server/          Spring Boot 3.4, Java 21, Gradle, PostgreSQL, JPA, Flyway
│   └── src/main/java/<group>/<app>/...   패키지는 feature 단위 (user/, bookmark/, tag/)
├── web/             Next.js 15 (App Router), TypeScript, Tailwind, TanStack Query
│   └── app/, components/, lib/
├── docs/
│   ├── spec/        기획 산출물 (planner)
│   ├── design/      화면 설계 (designer)
│   ├── api/         API 명세 (backend)
│   ├── api-usage/   FE의 API 사용 기록 (frontend)
│   └── review/      검수 리포트 (reviewer)
└── .devsquad/       팀원 정의 (이 폴더)
```

## 4. 팀 규칙 (모든 팀원 공통)

- 언어: 문서는 한국어, 코드·식별자·커밋 메시지는 영어.
- 기술 용어는 한글로 음차하지 않고 원어를 쓴다 ("Undo 로그", "Endpoint").
- 선행 팀원의 산출물에 없는 요구사항은 만들어 내지 않는다. 필요하면 `[확인 필요: …]`로 표시하고 Plan에 질문으로 올린다.
- 모든 산출물은 파일로 남기고, 파일 상단에 `> Task: <task-id> · 작성: <팀원> · 버전: v<n>` 한 줄을 둔다.
- 반려 후 재제출 시 산출물 상단에 `## 반려 반영 내역` 절을 두고 무엇이 바뀌었는지 적는다.

## 5. 도메인 용어집

<!-- TODO: 이 프로젝트에서만 쓰는 용어 5~15개. 팀원 간 용어 불일치를 막는다 -->

| 용어 | 의미 |
|---|---|
| 예: Bookmark | 사용자가 저장한 URL 1건. 제목·설명·태그·저장일을 가진다 |
| 예: Collection | Bookmark의 묶음. 사용자가 이름을 붙인다 |

## 6. 현재 알려진 제약

<!-- TODO: 성능 목표, 규제, 외부 시스템 의존, 하지 말아야 할 것 -->
