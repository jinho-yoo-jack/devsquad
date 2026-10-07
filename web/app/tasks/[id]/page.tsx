"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useMemo, useState } from "react";
import { useInitialEvents, useTask, useTaskApprovals, useTaskAction } from "@/lib/queries/tasks";
import { useTaskSocket } from "@/lib/ws/useTaskSocket";
import { useEventStore } from "@/lib/stores/eventStore";
import { useConnectionStore } from "@/lib/stores/connectionStore";
import { splitApprovals } from "@/lib/domain/approvals";
import { PipelineBar } from "@/components/pipeline/PipelineBar";
import { AgentActivityPanel } from "@/components/agents/AgentActivityPanel";
import { EventTimeline } from "@/components/timeline/EventTimeline";
import { ApprovalPanel } from "@/components/approval/ApprovalPanel";
import { StatusBadge } from "@/components/common/StatusBadge";
import { Button } from "@/components/common/Button";
import { InlineError } from "@/components/common/InlineError";

/** 12-디자인 §3.3 Task 상세 — 파이프라인(상단) / agent 카드 + 타임라인(좌) / 승인 패널 + 산출물(우). */
export default function TaskDetailPage() {
  const { id } = useParams<{ id: string }>();
  const task = useTask(id);
  const approvals = useTaskApprovals(id);
  const action = useTaskAction(id);
  const events = useInitialEvents(id);
  useTaskSocket(id);

  const bucket = useEventStore((s) => s.byTask[id]);
  const conn = useConnectionStore((s) => s.status);
  const [stageFilter, setStageFilter] = useState<string | null>(null);
  const selectStage = (key: string) => setStageFilter((cur) => cur === key ? null : key);

  const { pending, history } = useMemo(() => splitApprovals(approvals.data ?? [], task.data?.status), [approvals.data, task.data?.status]);
  const roleByStageId = useMemo(() => new Map<string, string>(), []);

  if (task.isPending) return <p className="text-sm" style={{ color: "var(--text-2)" }}>불러오는 중…</p>;
  if (task.isError || !task.data) return <InlineError message="Task를 불러오지 못했습니다." onRetry={() => task.refetch()} />;
  const t = task.data;

  return (
    <section className="space-y-4">
      <header className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-3">
          <Link href="/tasks" className="text-sm" style={{ color: "var(--text-2)" }}>← Tasks</Link>
          <h1 className="text-lg font-semibold">{t.command.split("\n")[0]}</h1>
          <StatusBadge status={t.status} />
        </div>
        <div className="flex items-center gap-2 text-xs" style={{ color: "var(--text-2)" }}>
          <span title="WebSocket">{conn === "open" ? "● 실시간" : conn === "reconnecting" ? "◐ 재연결 중" : "○ 오프라인"}</span>
          {t.status === "running" && <Button variant="ghost" onClick={() => action.mutate("pause")}>⏸ 일시정지</Button>}
          {(t.status === "paused" || t.status === "blocked") && <Button variant="ghost" onClick={() => action.mutate("resume")}>▶ 재개</Button>}
          {!["completed", "failed", "cancelled"].includes(t.status) && <Button variant="ghost" onClick={() => action.mutate("cancel")}>✖ 취소</Button>}
        </div>
      </header>

      <PipelineBar stages={t.stages} selected={stageFilter} onSelect={selectStage} />

      {events.isPending && <p role="status" className="text-xs" style={{ color: "var(--text-2)" }}>활동 기록을 불러오는 중…</p>}
      {events.isError && <InlineError message="활동 기록을 불러오지 못했습니다. 최근 작업 정보가 누락될 수 있습니다." onRetry={() => events.refetch()} />}
      <AgentActivityPanel task={t} events={bucket?.events ?? []} selected={stageFilter} onSelect={selectStage} live={conn === "open"} />

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_480px]">
        <div className="flex min-h-[60vh] flex-col gap-3">
          <div className="flex items-center justify-between text-xs" style={{ color: "var(--text-2)" }}>
            <span>타임라인 {stageFilter ? `· ${stageFilter}` : "· 전체 단계"}</span>
            {stageFilter && <button className="underline" onClick={() => setStageFilter(null)}>필터 해제</button>}
          </div>
          <div className="min-h-0 flex-1"><EventTimeline taskId={id} stageFilter={stageFilter} /></div>
        </div>

        <aside className="space-y-3">
          {pending.length === 0 && <div className="rounded-lg border p-4 text-sm" style={{ borderColor: "var(--border)", color: "var(--text-2)", background: "var(--surface-1)" }}>
            대기 중인 승인이 없습니다.
          </div>}
          {pending.map((a) => <ApprovalPanel key={a.id} approval={a} taskId={id} stageRole={t.stages.find((s) => s.key === (t.pending_approvals.find((p) => p.id === a.id)?.stage_key ?? ""))?.role ?? roleByStageId.get(a.stage_id)} />)}
          {history.length > 0 && (
            <details className="rounded-lg border" style={{ borderColor: "var(--border)", background: "var(--surface-1)" }}>
              <summary className="cursor-pointer px-3 py-2 text-sm">승인 이력 {history.length}건</summary>
              <div className="space-y-2 p-3">{history.map((a) => <ApprovalPanel key={a.id} approval={a} taskId={id} readOnly />)}</div>
            </details>
          )}
        </aside>
      </div>
    </section>
  );
}
