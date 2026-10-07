const format = (n: number) => n.toLocaleString("en-US");

/** 12-디자인 §4 TokenMeter — used / budget. */
export function TokenMeter({ used, budget, compact = false }: { used: number; budget: number; compact?: boolean }) {
  if (budget <= 0) return null;
  const ratio = Math.min(1, used / budget);
  const color = ratio >= 1 ? "var(--status-failed)" : ratio >= 0.8 ? "var(--status-waiting)" : "var(--status-running)";
  return (
    <div role="meter" aria-label="토큰 사용량" aria-valuemin={0} aria-valuemax={budget} aria-valuenow={used}
      className="flex items-center gap-2 text-xs" style={{ color: "var(--text-2)" }}>
      <span className={compact ? "h-1 w-12" : "h-1.5 w-24"} style={{ background: "var(--surface-2)", borderRadius: 9999, overflow: "hidden" }}>
        <span className="block h-full" style={{ width: `${ratio * 100}%`, background: color }} />
      </span>
      <span>{format(used)} / {format(budget)}{compact ? "" : " 토큰"}</span>
    </div>
  );
}
