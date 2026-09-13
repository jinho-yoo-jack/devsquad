"use client";

import Link from "next/link";
import { useState } from "react";
import { useTasks } from "@/lib/queries/tasks";
import type { TaskStatus } from "@/lib/api/schemas";
import { HealthBadge } from "@/components/common/HealthBadge";
import { EmptyState } from "@/components/common/EmptyState";
import { InlineError } from "@/components/common/InlineError";
import { TaskRow } from "@/components/tasks/TaskRow";
import { Button } from "@/components/common/Button";

const FILTERS: { label: string; statuses?: TaskStatus[] }[] = [
  { label: "전체" },
  { label: "진행 중", statuses: ["queued", "running"] },
  { label: "승인 대기", statuses: ["waiting_approval"] },
  { label: "차단", statuses: ["blocked", "paused"] },
  { label: "완료", statuses: ["completed"] },
  { label: "실패", statuses: ["failed", "cancelled"] },
];

export default function TasksPage() {
  const [filter, setFilter] = useState(0);
  const q = useTasks({ status: FILTERS[filter].statuses });

  return (
    <section className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Tasks</h1>
        <div className="flex items-center gap-3">
          <HealthBadge />
          <Link href="/tasks/new"><Button variant="primary">새 Task</Button></Link>
        </div>
      </div>
      <div className="flex flex-wrap gap-1.5" role="tablist">
        {FILTERS.map((f, i) => (
          <button key={f.label} role="tab" aria-selected={i === filter} onClick={() => setFilter(i)}
            className="rounded-full border px-3 py-1 text-xs"
            style={{ borderColor: i === filter ? "var(--status-running)" : "var(--border)", color: i === filter ? "var(--text-1)" : "var(--text-2)" }}>
            {f.label}
          </button>
        ))}
      </div>
      {q.isPending && <p className="text-sm" style={{ color: "var(--text-2)" }}>불러오는 중…</p>}
      {q.isError && <InlineError message="Task 목록을 불러오지 못했습니다." onRetry={() => q.refetch()} />}
      {q.data && q.data.items.length === 0 && (
        <EmptyState title="아직 Task가 없습니다" message="Discord에서 /task 로 시작하거나 여기서 새 Task를 만드세요."
          action={<Link href="/tasks/new"><Button variant="primary">첫 Task 만들기</Button></Link>} />
      )}
      <div className="space-y-2">{q.data?.items.map((t) => <TaskRow key={t.id} task={t} />)}</div>
    </section>
  );
}
