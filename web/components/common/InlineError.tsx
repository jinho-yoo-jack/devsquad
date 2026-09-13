export function InlineError({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div role="alert" className="flex items-center justify-between rounded-md border px-3 py-2 text-sm"
      style={{ borderColor: "var(--status-failed)", color: "var(--status-failed)" }}>
      <span>{message}</span>
      {onRetry && <button className="underline" onClick={onRetry}>다시 시도</button>}
    </div>
  );
}
