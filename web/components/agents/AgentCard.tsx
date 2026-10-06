"use client";

import type { Stage } from "@/lib/api/schemas";
import type { AgentActivity } from "@/lib/domain/activity";
import { clockTime, roleColor } from "@/lib/domain/status";

const colors: Record<AgentActivity["kind"], string> = {
  working: "var(--status-running)", waiting: "var(--status-waiting)", pending: "var(--text-2)",
  done: "var(--status-done)", blocked: "var(--status-rejected)", stopped: "var(--text-2)",
};

export function AgentCard({ stage, activity, onClick, selected, live }:
  { stage: Stage; activity: AgentActivity; onClick: () => void; selected: boolean; live: boolean }) {
  return (
    <button type="button" onClick={onClick} aria-pressed={selected}
      aria-label={`${stage.role} · ${stage.key} · ${activity.label} · 타임라인 보기`}
      className={`flex min-w-0 flex-col items-stretch rounded-lg border p-4 text-left ${live && activity.kind === "working" ? "pulse" : ""} ${selected ? "ring-1" : ""}`}
      style={{ borderColor: selected ? roleColor(stage.role) : "var(--border)", background: "var(--surface-1)" }}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="flex items-center gap-2 text-sm font-medium">
          <span className="inline-flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold"
            style={{ background: roleColor(stage.role), color: "#0B0F14" }} aria-hidden>{stage.role.slice(0, 2).toUpperCase()}</span>
          {stage.role}
        </span>
        <span className="text-xs" style={{ color: colors[activity.kind] }}>{activity.label}</span>
      </div>
      <p className="mt-2 truncate text-xs" style={{ color: "var(--text-2)" }} title={stage.key}>
        단계 {stage.key}{stage.retry_count > 0 ? ` · 재시도 ${stage.retry_count}회` : ""}
      </p>
      <p className="mt-3 text-sm font-medium">{activity.summary}</p>
      {activity.detail && <p className="mt-1 line-clamp-2 break-all text-xs" style={{ color: "var(--text-2)" }} title={activity.detail}>{activity.detail}</p>}
      {activity.lastTool && <p className="mt-2 truncate font-mono text-[11px]" style={{ color: "var(--text-2)" }}>최근 도구: {activity.lastTool}</p>}
      {activity.model && <p className="mt-1 truncate text-[11px]" style={{ color: "var(--text-2)" }} title={activity.model}>
        {activity.model}{activity.iteration ? ` · 호출 ${activity.iteration}회차` : ""}
      </p>}
      <p className="mt-3 text-[11px]" style={{ color: "var(--text-2)" }}>
        {activity.updatedAt ? <>마지막 활동 <time dateTime={activity.updatedAt} title={new Date(activity.updatedAt).toLocaleString()}>{clockTime(activity.updatedAt)}</time></> : "아직 수신된 활동이 없습니다."}
        {selected && <span className="ml-2">기록 표시 중</span>}
      </p>
    </button>
  );
}
