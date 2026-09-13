# designer knowledge

| 파일 | 내용 | 언제 |
|---|---|---|
| `design-tokens.md` | 색·간격·타이포·반경 토큰 이름과 의미 (값은 참고용) | 항상 |
| `component-inventory.md` | `web/components/`에 이미 있는 컴포넌트와 props 계약 | 항상 |
| (추가) `patterns/*.md` | 프로젝트의 UI 패턴 결정 (모달 vs 페이지, 삭제 확인 방식, 토스트 정책) | 관련 화면 설계 시 |

## 유지 규칙
- `component-inventory.md`는 프론트엔드 팀원이 새 컴포넌트를 만들 때마다 갱신한다 (frontend conventions §3 참고).
- 토큰 파일은 `web/styles/tokens.css`와 동기화한다. 값이 바뀌면 이 파일도 바꾼다.
