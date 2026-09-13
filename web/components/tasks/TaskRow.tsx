"use client";

import Link from "next/link";
import type { Task } from "@/lib/api/schemas";
import { relativeTime, stageStatusMeta } from "@/lib/domain/status";
import { StatusBadge } from "@/components/common/StatusBadge";

/** 12-디자인 §3.1 — 행 목록. 승인 대기 행은 왼쪽 테두리 강조. */
export function TaskRow({ task }: { task: Task }) {
  const waiting = task.status === "waiting_approval";
  return (
    <Link href={`/tasks/${task.id}`} className="block rounded-md border px-4 py-3 hover:bg-[var(--surface-2)]"
      style={{ borderColor: "var(--border)", background: "var(--surface-1)", borderLeft: waiting ? "3px solid var(--status-waiting)" : undefined }}>
      <div className="flex items-center justify-between gap-3">
        <p className="truncate font-medium">{task.command.split("\n")[0]}</p>
        <div className="flex shrink-0 items-center gap-2">
          {task.pending_approvals.length > 0 && (
            <span className="rounded-full px-2 py-0.5 text-xs font-semibold" style={{ background: "var(--status-waiting)", color: "#0B0F14" }}>
              승인 {task.pending_approvals.length}
            </span>
          )}
          <StatusBadge status={task.status} />
        </div>
      </div>
      <div className="mt-2 flex items-center justify-between text-xs" style={{ color: "var(--text-2)" }}>
        <div className="flex items-center gap-1" aria-label="미니 파이프라인">
          {task.stages.map((s) => (
            <span key={s.key} title={`${s.key} · ${stageStatusMeta[s.status].label}`} style={{ color: stageStatusMeta[s.status].color }}>
              {stageStatusMeta[s.status].icon}
            </span>
          ))}
        </div>
        <span>
          {task.last_event ? `${task.last_event.type} · ` : ""}{relativeTime(task.updated_at)}
        </span>
      </div>
    </Link>
  );
}
