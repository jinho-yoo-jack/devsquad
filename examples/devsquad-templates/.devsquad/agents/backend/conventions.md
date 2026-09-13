# backend — 행동 규약

# 1. 입력 (Inputs)
- 승인된 `docs/spec/<task-slug>.md` — §4 UC, §6 데이터 항목, `devsquad:summary.entities`.
- 승인된 `docs/design/<task-slug>.md` + `.components.json` — 화면이 어떤 데이터를 언제 필요로 하는지 (Endpoint 설계 근거).
- `knowledge/api-guidelines.md`, `knowledge/db-schema.md`, `knowledge/error-codes.md`.
- `server/` 기존 코드 — 같은 도메인 패키지가 있으면 그 구조를 따른다. 작업 시작 시 관련 패키지를 반드시 읽는다.
- `docs/api/` 기존 명세 — 경로·명명 규칙을 잇는다.

# 2. Plan 형식 (Plan format)
```
## 백엔드 계획 — <task-slug>
1. Endpoint (예상):
   | 메서드 | 경로 | UC | 비고 |
2. 도메인 변경: 엔티티 <이름> 추가/수정, 마이그레이션 V<n>__<설명>.sql
3. 패키지: server/src/main/java/<...>/<feature>/
4. 테스트: 단위 <n>개 (서비스), 통합 <n>개 (Controller + Testcontainers)
5. 명세 파일: docs/api/<task-slug>.md
6. 확인이 필요한 질문: (없으면 "없음")
```

# 3. 작업 규칙 (Working rules)
- **순서 고정**: (1) `docs/api/<task-slug>.md` 작성 → (2) Flyway 마이그레이션 → (3) 도메인·리포지토리 → (4) 서비스 + 단위 테스트 → (5) Controller + 통합 테스트 → (6) `test_command` 실행 → (7) 커밋.
- 패키지는 feature 단위: `<feature>/{api,app,domain,infra}`. 기존 코드가 다른 구조면 기존을 따른다.
- DTO는 `record`. 요청 DTO는 `*Request`, 응답은 `*Response`. 엔티티를 Controller 밖으로 내보내지 않는다.
- 검증은 Bean Validation(`@Valid`), 도메인 규칙 위반은 도메인 예외(`<Feature>Exception` 상속) → `GlobalExceptionHandler`가 `knowledge/error-codes.md` 형식으로 변환.
- 트랜잭션 경계는 서비스 메서드. Controller에 `@Transactional` 금지.
- 마이그레이션 파일명 `V<yyyyMMddHHmm>__<snake_case>.sql`. 기존 컬럼 삭제·타입 변경 금지(제안으로).
- 테스트: 서비스는 Mockito 없이 가능한 한 순수 단위(도메인) + 슬라이스, Controller는 `@SpringBootTest` + Testcontainers Postgres. 테스트 이름은 `should_<기대>_when_<조건>`.
- 커밋 메시지: `feat(<feature>): <요약>` / `test(<feature>): …` / `docs(api): …`. 한 Stage에 커밋 여러 개 가능.
- 로그에 개인정보·토큰을 남기지 않는다.

# 4. 결과물 형식 (Deliverable format)

## 4.1 `docs/api/<task-slug>.md` — 프론트엔드와 검수자가 계약으로 사용
```markdown
> Task: <task-id> · 작성: backend · 버전: v<n>

# <기능 이름> API 명세

## 1. 개요 (관련 UC, 인증 방식)
## 2. Endpoint
### <METHOD> <path>
- UC: · 권한:
- 요청: (path/query/body — 필드, 타입, 필수, 제약)
- 응답 200/201: (필드, 타입, 예시 JSON)
- 오류: | HTTP | code | 조건 |
## 3. 데이터 변경 (마이그레이션 요약)
## 4. 비고 (멱등성, 페이지네이션, 정렬 기본값)
```
문서 끝에 기계 판독 블록(OpenAPI 3 조각). 검수자가 이것으로 FE 호출을 대조한다:
```yaml
# devsquad:openapi
paths:
  /api/v1/users/me/withdrawal:
    post:
      operationId: requestWithdrawal
      requestBody: { required: true, content: { application/json: { schema: { $ref: '#/components/schemas/WithdrawalRequest' } } } }
      responses:
        '202': { description: accepted, content: { application/json: { schema: { $ref: '#/components/schemas/WithdrawalResponse' } } } }
        '409': { description: already requested }
components:
  schemas:
    WithdrawalRequest: { type: object, required: [reason], properties: { reason: { type: string, maxLength: 500 } } }
    WithdrawalResponse: { type: object, properties: { scheduledAt: { type: string, format: date-time } } }
```

## 4.2 코드 커밋
브랜치 `devsquad/<task-id>/backend`(서비스가 만든다). Deliverable 요약에는 변경 파일 목록, 테스트 결과(통과/실패 수), 명세 파일 경로를 적는다.

# 5. 완료 기준 (Definition of done)
- [ ] `docs/api/<task-slug>.md`의 모든 Endpoint가 구현되어 있고, 구현된 모든 Endpoint가 명세에 있다.
- [ ] `devsquad:openapi` 블록이 유효한 YAML이고 본문과 일치한다.
- [ ] `test_command`가 통과한다 (실패 시 결과를 그대로 적고 승인 요청).
- [ ] 새 서비스 메서드마다 단위 테스트가 있고, 새 Endpoint마다 통합 테스트가 최소 정상 1 + 오류 1 있다.
- [ ] 마이그레이션이 되돌릴 수 있다 (파괴적 변경 없음).
- [ ] 엔티티가 응답으로 노출된 곳이 없다.
- [ ] 재제출이면 `## 반려 반영 내역`이 있다.

# 6. 금지 사항 (Never)
- `web/`를 수정하지 않는다. FE에 필요한 변경은 Deliverable 비고에 적는다.
- 명세에 없는 Endpoint를 구현하거나, 구현 없이 명세만 남기지 않는다.
- `ddl-auto=update`, 컬럼 삭제, 운영 데이터 변경 SQL을 쓰지 않는다.
- 시크릿·접속 정보를 코드나 테스트에 하드코딩하지 않는다. `.env*`는 읽지도 않는다.
- 외부 네트워크를 호출하는 테스트를 쓰지 않는다.
