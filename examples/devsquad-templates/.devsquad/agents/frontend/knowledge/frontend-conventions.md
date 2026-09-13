# 프론트엔드 컨벤션 상세

## 디렉토리
```
web/
├── app/                      # 라우트. page.tsx 는 서버 컴포넌트 기본, 상호작용은 하위 클라이언트 컴포넌트로
├── components/
│   ├── ui/                   # 디자인 시스템 primitive (shadcn). 직접 수정 최소화
│   ├── common/               # 프로젝트 공통 (EmptyState, ConfirmDialog …)
│   └── <feature>/            # 기능별
├── lib/
│   ├── api/<feature>.ts      # fetch 래퍼 + operationId 함수
│   ├── queries/<feature>.ts  # useQuery / useMutation hook
│   ├── schemas/<feature>.ts  # zod 스키마 + 타입
│   └── utils/
└── tests/
```

## API 계층 예시
```ts
// lib/schemas/account.ts
export const WithdrawalRequest = z.object({ reason: z.string().min(1).max(500) });
export const WithdrawalResponse = z.object({ scheduledAt: z.string().datetime() });
export type WithdrawalRequest = z.infer<typeof WithdrawalRequest>;

// lib/api/account.ts
export async function requestWithdrawal(body: WithdrawalRequest) {
  return apiClient.post('/api/v1/users/me/withdrawal', body, WithdrawalResponse);
}

// lib/queries/account.ts
export function useRequestWithdrawal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: requestWithdrawal,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['account'] }),
  });
}
```
`apiClient.post(path, body, responseSchema)`는 응답을 `responseSchema.parse`로 검증하고 오류 응답을 `ApiError(code, message, details)`로 던진다.

## 상태 처리 패턴
```tsx
const { data, isPending, isError, refetch } = useBookmarks(params);
if (isPending) return <Skeleton className="h-24" />;
if (isError) return <InlineError message="목록을 불러오지 못했습니다." onRetry={refetch} />;
if (data.items.length === 0) return <EmptyState title="저장된 북마크가 없습니다" action={...} />;
```

## 폼
react-hook-form + `zodResolver(schema)`. 제출 중 버튼 `loading`, 서버 `VALIDATION_FAILED`의 `details.fields[]`는 `setError`로 필드에 매핑.

## 테스트
- 컴포넌트: 상태별 렌더 스냅샷 대신 역할 기반 쿼리(`getByRole`)로 검증.
- API mock: msw 핸들러를 `tests/msw/<feature>.ts`에. 명세의 예시 JSON을 그대로 사용.
- 파일명 `<Component>.test.tsx`, `<hook>.test.ts`.

## 금지
`any`, `@ts-ignore`, 컴포넌트 내 직접 `fetch`, 인라인 hex 색, `useEffect`로 데이터 fetch.
