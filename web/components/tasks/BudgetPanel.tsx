"use client";

import { useState } from "react";
import { Button } from "@/components/common/Button";

/** 예산 초과 일시정지 안내와 증액. 증액만으로는 재개하지 않으며 재개는 헤더의 ▶ 재개로 한다. */
export function BudgetPanel({ used, budget, onSubmit, busy = false, error }: { used: number; budget: number; onSubmit: (budget: number) => void; busy?: boolean; error?: string }) {
  const [value, setValue] = useState(String(Math.max(budget * 2, used + 1)));
  const next = Number(value);
  const valid = Number.isInteger(next) && next > used;
  return (
    <section className="space-y-2 rounded-lg border p-3 text-sm" style={{ borderColor: "var(--status-waiting)", background: "var(--surface-1)" }}>
      <p role="status">토큰 예산({budget.toLocaleString("en-US")})을 모두 사용해 일시정지되었습니다. 예산을 늘린 뒤 재개하세요.</p>
      <div className="flex flex-wrap items-center gap-2">
        <input type="number" min={used + 1} step={1} value={value} onChange={(e) => setValue(e.target.value)} aria-label="새 토큰 예산"
          className="w-40 rounded-md border px-2 py-1" style={{ borderColor: "var(--border)", background: "var(--bg)", color: "var(--text-1)" }} />
        <Button variant="primary" loading={busy} disabled={!valid} onClick={() => onSubmit(next)}>예산 변경</Button>
      </div>
      {error && <p role="alert" className="text-xs" style={{ color: "var(--status-failed)" }}>{error}</p>}
    </section>
  );
}
