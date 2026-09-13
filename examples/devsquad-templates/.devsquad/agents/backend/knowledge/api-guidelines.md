# API 가이드라인

## URL
- prefix `/api/v1`. 리소스는 복수 명사 kebab-case: `/api/v1/bookmarks`, `/api/v1/bookmarks/{id}/tags`.
- 행위가 리소스로 표현되지 않으면 하위 리소스 명사로: `POST /users/me/withdrawal` (○), `POST /users/withdraw` (✕).
- 본인 리소스는 `/users/me/...`.

## 메서드와 상태 코드
| 상황 | 메서드 | 성공 |
|---|---|---|
| 생성 | POST | 201 + Location |
| 비동기 접수 | POST | 202 |
| 전체 수정 | PUT | 200 |
| 부분 수정 | PATCH | 200 |
| 삭제 | DELETE | 204 |
| 조회 | GET | 200 |

## 응답 형식
- 단건: 객체 그대로. 목록: `{ "items": [...], "nextCursor": "..." }` 커서 방식. offset 페이지네이션 금지.
- 시간은 ISO-8601 UTC(`2026-09-11T01:00:00Z`). 필드명 camelCase.
- null 필드는 생략하지 않고 `null`로 내려 스키마를 고정한다.

## 멱등성
- POST 생성에 클라이언트 `Idempotency-Key` 헤더를 받는 경우 24시간 동일 응답.
- PUT/DELETE는 자연 멱등.

## 오류
`error-codes.md` 형식. 400 검증 오류는 `details.fields[]`에 필드별 메시지.

## 버저닝
경로 버전(`/v1`). 하위 호환 깨지는 변경은 새 필드 추가로 우회하고, 정말 필요하면 v2 Endpoint 추가(기존 유지).
