"use client";

import { useMemo } from "react";
import type { Task } from "@/lib/api/schemas";
import type { TaskEvent } from "@/lib/domain/events";
import { agentActivity, indexStageActivity } from "@/lib/domain/activity";
import { layoutStages } from "@/lib/domain/pipeline";
import { AgentCard } from "./AgentCard";

export function AgentActivityPanel({ task, events, selected, onSelect, live }:
  { task: Task; events: TaskEvent[]; selected: string | null; onSelect: (key: string) => void; live: boolean }) {
  const logs = useMemo(() => indexStageActivity(events), [events]);
  const cards = layoutStages(task.stages).flat().map((stage) => ({ stage, activity: agentActivity(task, stage, logs[stage.key]) }));
  const count = (kind: string) => cards.filter((c) => c.activity.kind === kind).length;
  return (
    <section aria-label="에이전트 작업 현황" className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-sm font-semibold">에이전트 작업 현황</h2>
        <p className="text-xs" style={{ color: "var(--text-2)" }}>
          작업 중 {count("working")} · 승인 대기 {count("waiting")} · 완료 {count("done")} / {cards.length}단계
        </p>
      </div>
      {!live && <p role="status" className="text-xs" style={{ color: "var(--status-waiting)" }}>
        실시간 연결 대기 중입니다. 마지막으로 수신한 활동을 표시하며 연결되면 갱신합니다.
      </p>}
      {cards.length ? <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {cards.map(({ stage, activity }) => <AgentCard key={stage.key} stage={stage} activity={activity}
          selected={selected === stage.key} onClick={() => onSelect(stage.key)} live={live} />)}
      </div> : <p className="text-sm" style={{ color: "var(--text-2)" }}>등록된 에이전트 단계가 없습니다.</p>}
      <p className="text-xs" style={{ color: "var(--text-2)" }}>카드를 선택하면 해당 단계의 작업 기록을 확인할 수 있습니다.</p>
    </section>
  );
}
