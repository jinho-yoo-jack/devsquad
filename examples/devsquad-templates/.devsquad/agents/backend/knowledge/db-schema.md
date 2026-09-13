# DB 스키마 요약

> 실제 프로젝트에서는 `./gradlew flywayInfo` 결과나 `pg_dump --schema-only` 출력을 정리해 넣는다.
> 접속 정보·호스트는 절대 넣지 않는다. 스키마만.

## 공통 규칙
- 모든 테이블: `id uuid pk default gen_random_uuid()`, `created_at timestamptz not null default now()`, `updated_at timestamptz`.
- Soft delete가 필요한 테이블은 `deleted_at timestamptz null`. 조회 기본은 `deleted_at is null`.
- FK는 `on delete restrict` 기본. cascade는 명세에 이유를 적을 때만.
- 인덱스 명명 `ix_<table>_<cols>`, unique `ux_<table>_<cols>`.

## 테이블 (예시 — TODO: 실제로 교체)
### users
| 컬럼 | 타입 | 비고 |
|---|---|---|
| id | uuid | pk |
| email | text | ux |
| status | text | ACTIVE / WITHDRAWAL_PENDING / WITHDRAWN |
| withdrawal_scheduled_at | timestamptz | null |

### bookmarks
| 컬럼 | 타입 | 비고 |
|---|---|---|
| id | uuid | pk |
| user_id | uuid | fk users |
| url | text | |
| title | text | |
| collection_id | uuid | fk collections, null |
| deleted_at | timestamptz | soft delete |

## 관계
users 1—* bookmarks · users 1—* collections · collections 1—* bookmarks
