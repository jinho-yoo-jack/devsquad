"use client";

import type { Stage } from "@/lib/api/schemas";
import { layoutStages } from "@/lib/domain/pipeline";
import { roleColor, stageStatusMeta } from "@/lib/domain/status";

/** 12-디자인 §3.3 — 열 = super-step, 세로 = 병렬. */
export function PipelineBar({ stages, onSelect, selected }: { stages: Stage[]; onSelect?: (key: string) => void; selected?: string | null }) {
  const cols = layoutStages(stages);
  if (!cols.length) return null;
  return (
    <div className="flex items-start gap-2 overflow-x-auto py-2" role="list" aria-label="파이프라인">
      {cols.map((col, i) => (
        <div key={i} className="flex items-center gap-2">
          <div className="flex flex-col gap-1.5">
            {col.map((s) => {
              const m = stageStatusMeta[s.status];
              const active = s.status === "planning" || s.status === "executing";
              return (
                <button key={s.key} role="listitem" onClick={() => onSelect?.(s.key)}
                  title={`${s.key} · ${m.label}${s.retry_count ? ` · 반려 ${s.retry_count}회` : ""}`}
                  className={`flex items-center gap-2 rounded-md border px-2.5 py-1 text-xs ${selected === s.key ? "ring-1" : ""} ${active ? "pulse" : ""}`}
                  style={{ borderColor: m.color, background: "var(--surface-1)", color: "var(--text-1)" }}>
                  <span className="inline-block h-2 w-2 rounded-full" style={{ background: roleColor(s.role) }} aria-hidden />
                  <span className="font-medium">{s.key}</span>
                  <span style={{ color: m.color }} aria-label={m.label}>{m.icon}</span>
                  {s.retry_count > 0 && <span style={{ color: "var(--status-rejected)" }}>⟲{s.retry_count}</span>}
                </button>
              );
            })}
          </div>
          {i < cols.length - 1 && <span aria-hidden style={{ color: "var(--border)" }}>───</span>}
        </div>
      ))}
    </div>
  );
}
