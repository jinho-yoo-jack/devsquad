"use client";

import type { Stage } from "@/lib/api/schemas";
import { roleColor, stageStatusMeta } from "@/lib/domain/status";

export function AgentCard({ stage, thinking, lastTool, onClick, selected }:
  { stage: Stage; thinking?: string; lastTool?: string; onClick?: () => void; selected?: boolean }) {
  const m = stageStatusMeta[stage.status];
  const active = stage.status === "planning" || stage.status === "executing";
  return (
    <button onClick={onClick}
      className={`min-w-[180px] flex-1 rounded-lg border p-3 text-left ${active ? "pulse" : ""} ${selected ? "ring-1" : ""}`}
      style={{ borderColor: selected ? roleColor(stage.role) : "var(--border)", background: "var(--surface-1)" }}>
      <div className="flex items-center justify-between">
        <span className="flex items-center gap-2 text-sm font-medium">
          <span className="inline-flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold"
            style={{ background: roleColor(stage.role), color: "#0B0F14" }} aria-hidden>{stage.role.slice(0, 2).toUpperCase()}</span>
          {stage.role}
        </span>
        <span className="text-xs" style={{ color: m.color }}>{m.icon} {m.label}</span>
      </div>
      <p className="mt-2 line-clamp-2 min-h-[2.5rem] text-xs" style={{ color: thinking ? "var(--text-1)" : "var(--text-2)" }}>
        {thinking ? `💭 ${thinking}` : active ? "…" : "—"}
      </p>
      {lastTool && <p className="mt-1 truncate text-[11px]" style={{ color: "var(--text-2)" }}>🔧 {lastTool}</p>}
    </button>
  );
}
