"use client";

import { useState } from "react";
import type { TaskEvent } from "@/lib/domain/events";
import { clockTime, eventIcon, roleColor } from "@/lib/domain/status";
import type { ToolPair } from "@/lib/stores/eventStore";

function summary(ev: TaskEvent): string {
  const p = ev.payload as Record<string, unknown>;
  switch (ev.type) {
    case "agent.thinking": return String(p.summary ?? "");
    case "agent.message": return String(p.text ?? "");
    case "agent.tool_call": return `${p.tool} ${p.args_summary ?? ""}`;
    case "agent.tool_result": return `${p.tool} → ${p.ok ? "ok" : "denied"} ${p.summary ?? ""}`;
    case "approval.requested": return `승인 요청 · ${p.title ?? p.kind}`;
    case "approval.decided": return `${p.decision} (${p.decided_via ?? "?"})`;
    case "deliverable.produced": return `산출물 ${p.uri ?? p.kind ?? ""}`;
    case "usage": return `${p.model} in ${p.input_tokens} / out ${p.output_tokens}`;
    case "stage.blocked": return `차단 — ${p.reason ?? ""}`;
    case "run.failed": return `실패 — ${p.error ?? ""}`;
    default: return ev.type;
  }
}

export function EventRow({ ev, pair }: { ev: TaskEvent; pair?: ToolPair }) {
  const [open, setOpen] = useState(false);
  const isPair = !!pair && ev.type === "agent.tool_call";
  const ok = pair?.result ? (pair.result.payload as Record<string, unknown>).ok !== false : undefined;
  return (
    <div className="flex gap-2 px-2 py-1 text-[13px] hover:bg-[var(--surface-2)]" style={{ borderBottom: "1px solid var(--border)" }}>
      <span className="w-[62px] shrink-0 font-mono text-[11px]" style={{ color: "var(--text-2)" }}>{clockTime(ev.ts)}</span>
      <span className="inline-block h-2 w-2 shrink-0 self-center rounded-full" style={{ background: roleColor(ev.agent) }} aria-hidden />
      <span className="w-[72px] shrink-0 truncate text-[11px]" style={{ color: "var(--text-2)" }}>{ev.agent ?? "system"}</span>
      <span className="shrink-0" aria-hidden>{eventIcon[ev.type] ?? "•"}</span>
      <div className="min-w-0 flex-1">
        <button className="w-full truncate text-left" onClick={() => setOpen((o) => !o)} title={summary(ev)}>
          {summary(ev)}
          {isPair && <span className="ml-2 text-[11px]" style={{ color: ok === undefined ? "var(--text-2)" : ok ? "var(--status-done)" : "var(--status-failed)" }}>
            {ok === undefined ? "⏳" : ok ? "✓" : "✗"}
          </span>}
        </button>
        {open && (
          <pre className="mt-1 max-h-60 overflow-auto rounded p-2 text-[11px]" style={{ background: "var(--bg)", color: "var(--text-2)" }}>
            {JSON.stringify(isPair ? { call: ev.payload, result: pair?.result?.payload } : ev.payload, null, 2)}
          </pre>
        )}
      </div>
    </div>
  );
}
