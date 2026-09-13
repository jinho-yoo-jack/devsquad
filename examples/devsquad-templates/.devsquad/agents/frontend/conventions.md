# frontend — 행동 규약

# 1. 입력 (Inputs)
- 승인된 `docs/design/<task-slug>.md` + `docs/design/<task-slug>.components.json` — 구조의 기준.
- `docs/api/<task-slug>.md`의 `devsquad:openapi` 블록 — 호출의 기준. **주의: 백엔드와 병렬 실행되므로 이 파일이 아직 없을 수 있다.** 없으면 spec §6 데이터 항목과 화면 설계로 필요한 호출을 추정해 `lib/api/<feature>.ts`에 타입과 함수를 만들고, `docs/api-usage`에 "[명세 부재]"로 표시한다. 검수자가 대조한다.
- 승인된 `docs/spec/<task-slug>.md` — 카피·검증 규칙의 근거.
- `knowledge/frontend-conventions.md`, designer의 `knowledge/component-inventory.md`.
- `web/` 기존 코드 — 라우팅·상태·API 계층 구조를 따른다.

# 2. Plan 형식 (Plan format)
```
## 프론트엔드 계획 — <task-slug>
1. 라우트/화면: /path → SCR-xx
2. 컴포넌트: 재사용 <목록> / 신규 <목록> (components.json 기준)
3. API 호출: <operationId 또는 METHOD path> × n  (명세 있음/없음 표시)
4. 상태: 서버 상태 (query key) / 클라이언트 상태
5. 테스트: 컴포넌트 <n>개, hook <n>개
6. 확인이 필요한 질문: (없으면 "없음")
```

# 3. 작업 규칙 (Working rules)
- 디렉토리: `app/<route>/page.tsx`(서버 컴포넌트 기본), `components/<feature>/*.tsx`, `lib/api/<feature>.ts`, `lib/queries/<feature>.ts`, `lib/schemas/<feature>.ts`(zod).
- API 계층: `lib/api/<feature>.ts`에 명세의 operationId 이름으로 함수. 요청/응답 타입은 zod 스키마에서 `z.infer`. 컴포넌트에서 fetch 직접 호출 금지.
- 서버 상태는 TanStack Query. 쿼리 키 `['<feature>', ...]`. 변경은 `useMutation` + 관련 키 무효화.
- 폼은 react-hook-form + zod. 검증 메시지는 spec의 규칙을 그대로.
- 모든 화면에 로딩(`Skeleton`)·빈(`EmptyState`)·오류(`InlineError` + 재시도) 처리.
- 스타일은 Tailwind + 토큰 CSS 변수. 임의 hex/px 금지. `className` 조합은 `cn()`.
- 접근성: 인터랙티브 요소는 키보드 접근 가능, 모달은 포커스 트랩, 아이콘 버튼은 `aria-label`.
- 새 컴포넌트를 만들면 `.devsquad/agents/designer/knowledge/component-inventory.md`에 한 줄 추가한다.
- 테스트: Vitest + Testing Library. 컴포넌트는 상태별 렌더 1개 이상, hook은 성공/오류 경로. API는 msw로 mock.
- 커밋: `feat(web/<feature>): …`, `test(web/<feature>): …`.

# 4. 결과물 형식 (Deliverable format)

## 4.1 코드 커밋
브랜치 `devsquad/<task-id>/frontend`. Deliverable 요약: 변경 파일 목록, 신규 컴포넌트, `test_command` 결과.

## 4.2 `docs/api-usage/<task-slug>.md` — 검수자가 백엔드 명세와 대조
```markdown
> Task: <task-id> · 작성: frontend · 버전: v<n>

# <기능 이름> API 사용 기록

| operationId / 경로 | 메서드 | 호출 위치 (파일:함수) | 요청 필드 | 응답 필드 사용 | 명세 상태 |
|---|---|---|---|---|---|
| requestWithdrawal `/api/v1/users/me/withdrawal` | POST | lib/api/account.ts:requestWithdrawal | reason | scheduledAt | 명세 있음 v1 |
| … | | | | | [명세 부재] 추정 |

## 명세와 다른 점 / 추정한 점
- …
```
문서 끝에 기계 판독 블록:
```yaml
# devsquad:api-usage
calls:
  - operationId: requestWithdrawal
    method: POST
    path: /api/v1/users/me/withdrawal
    request: [reason]
    response: [scheduledAt]
    specStatus: present   # present | missing | mismatch
```

# 5. 완료 기준 (Definition of done)
- [ ] `components.json`의 모든 screen이 라우트로 존재하고, 모든 component가 구현되어 있다.
- [ ] `test_command`(typecheck + test)가 통과한다.
- [ ] 모든 화면에 로딩·빈·오류 상태가 있다.
- [ ] `docs/api-usage/<task-slug>.md`가 실제 코드의 호출과 일치한다 (`devsquad:api-usage` 블록 포함).
- [ ] 신규 컴포넌트가 인벤토리에 등록되었다.
- [ ] 임의 색·간격 값이 없다.
- [ ] 재제출이면 `## 반려 반영 내역`이 있다.

# 6. 금지 사항 (Never)
- `server/`를 수정하지 않는다. 필요한 API 변경은 api-usage 문서에 적는다.
- 명세에 없는 필드를 응답에서 읽거나 요청에 보내지 않는다 (추정은 [명세 부재]로만).
- `any` 타입, `// @ts-ignore`를 쓰지 않는다.
- 컴포넌트 트리 JSON의 구조를 임의로 바꾸지 않는다.
- 시크릿을 클라이언트 번들에 넣지 않는다 (`NEXT_PUBLIC_` 이외 env 접근 금지).
