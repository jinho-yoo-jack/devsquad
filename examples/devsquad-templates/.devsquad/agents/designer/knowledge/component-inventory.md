# 컴포넌트 인벤토리 (web/components/)

> 프론트엔드 팀원이 컴포넌트를 추가·변경할 때 이 파일을 갱신한다.

| 컴포넌트 | 경로 | props (핵심) | 상태 | 비고 |
|---|---|---|---|---|
| `Button` | ui/button | variant: primary·secondary·ghost·danger, size, loading, disabled | — | shadcn 기반 |
| `Input`, `Textarea` | ui/input | value, onChange, error, helperText | error | |
| `Dialog` | ui/dialog | open, onOpenChange, title, description | — | 모달의 기본 |
| `ConfirmDialog` | common/ConfirmDialog | title, message, confirmLabel, danger, onConfirm | submitting | 파괴적 동작 확인 |
| `SectionCard` | common/SectionCard | title, description, actions | — | 설정 페이지 섹션 |
| `EmptyState` | common/EmptyState | icon, title, message, action | — | |
| `InlineError` | common/InlineError | message, onRetry | — | |
| `Skeleton` | ui/skeleton | className | — | 로딩 |
| `Toast` | ui/toast (hook: useToast) | title, description, variant | — | 전역 알림 |
| `DataTable` | common/DataTable | columns, rows, emptyState, loading | loading, empty | |
| `Tabs` | ui/tabs | value, onValueChange, items | — | |
| `Badge` | ui/badge | variant | — | |
