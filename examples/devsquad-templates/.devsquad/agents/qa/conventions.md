# qa — 행동 규약 (확장 예시)

# 1. 입력 (Inputs)
- 승인된 `docs/spec/<task-slug>.md` §4 UC — 시나리오의 유일한 출처.
- 승인된 `docs/design/<task-slug>.md` — 화면 경로·카피(셀렉터 라벨).
- FE·BE 브랜치 (실행 대상). 실행 환경은 `knowledge/e2e-env.md`.
- `web/e2e/` 기존 시나리오 — fixture·헬퍼 재사용.

# 2. Plan 형식
```
## QA 계획 — <task-slug>
1. 자동화할 UC: UC-xxx (주 흐름 + E1), UC-yyy (주 흐름) …
2. 자동화하지 않을 UC와 이유:
3. 파일: web/e2e/<task-slug>.spec.ts
4. 필요한 fixture / 테스트 데이터:
```

# 3. 작업 규칙
- 파일 `web/e2e/<task-slug>.spec.ts`. `test.describe('UC-xxx <제목>')` 아래 주 흐름 1개 + 예외 흐름 n개.
- 셀렉터는 `getByRole` / `getByLabel` / `getByText`(카피 그대로)만. `data-testid`는 FE에 요청(리포트에 기록)하고 임시로도 CSS 셀렉터를 쓰지 않는다.
- 테스트 데이터는 `beforeEach`에서 API로 생성, `afterEach`에서 삭제.
- 고정 `waitForTimeout` 금지. `expect(...).toBeVisible()` 등 조건 대기.
- 실패 시 스크린샷·trace 저장 (`playwright.config`의 `on-first-retry`).

# 4. 결과물 형식
## 4.1 코드: `web/e2e/<task-slug>.spec.ts` (브랜치 `devsquad/<task-id>/qa`)
## 4.2 `docs/qa/<task-slug>.md`
```markdown
> Task: <task-id> · 작성: qa · 버전: v<n>

# <기능 이름> E2E 결과

| UC | 시나리오 | 결과 | 비고 (실패 시 재현 단계, 스크린샷 경로) |

## 자동화 제외 UC와 이유
## FE/BE에 요청하는 것 (testid, 시드 데이터 등)
```
```yaml
# devsquad:qa
passed: 5
failed: 1
failed_ucs: [UC-003]
```

# 5. 완료 기준
- [ ] 계획한 모든 UC에 시나리오가 있고 `test_command`가 실행되었다 (실패가 있어도 결과를 그대로 보고).
- [ ] CSS 셀렉터·고정 대기 없음.
- [ ] 결과 표의 UC가 spec의 UC 번호와 일치한다.

# 6. 금지 사항
- `web/e2e/`, `docs/qa/` 밖을 수정하지 않는다.
- 실패한 테스트를 `skip`으로 숨기지 않는다.
