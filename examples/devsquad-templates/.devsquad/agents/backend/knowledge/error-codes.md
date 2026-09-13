# 오류 응답 형식과 코드

## 형식
```json
{
  "code": "BOOKMARK_NOT_FOUND",
  "message": "북마크를 찾을 수 없습니다.",
  "details": { "bookmarkId": "..." },
  "traceId": "..."
}
```
`code`는 `UPPER_SNAKE`, `<도메인>_<상황>`. 메시지는 사용자에게 그대로 보여도 되는 한국어 문장.

## 공통 코드
| HTTP | code | 조건 |
|---|---|---|
| 400 | `VALIDATION_FAILED` | Bean Validation 실패. `details.fields[]` |
| 401 | `UNAUTHENTICATED` | 토큰 없음/만료 |
| 403 | `FORBIDDEN` | 본인 리소스 아님 |
| 404 | `<RESOURCE>_NOT_FOUND` | |
| 409 | `<RESOURCE>_CONFLICT` 또는 상황별 (`WITHDRAWAL_ALREADY_REQUESTED`) | 상태 충돌 |
| 422 | `<DOMAIN>_RULE_VIOLATION` | 도메인 규칙 위반 (예: 한도 초과) |
| 500 | `INTERNAL_ERROR` | 메시지에 내부 정보 노출 금지 |

## 구현
- 도메인 예외는 `abstract class DomainException(code, httpStatus, message)`를 상속.
- `GlobalExceptionHandler`(`@RestControllerAdvice`)가 위 형식으로 변환. Controller에서 try/catch 금지.
- 새 코드를 추가하면 이 문서의 도메인별 절에 등록한다.

## 도메인별 코드
<!-- TODO: 프로젝트별로 채운다 -->
| HTTP | code | 조건 |
|---|---|---|
