"use client";

import { useVirtualizer } from "@tanstack/react-virtual";
import { useEffect, useMemo, useRef, useState } from "react";
import type { TaskEvent } from "@/lib/domain/events";
import { useEventStore } from "@/lib/stores/eventStore";
import { EventRow } from "./EventRow";

const HIDDEN_TYPES = new Set(["usage"]); // 비용은 별도 미터. 타임라인 소음 방지.

/** 12-디자인 §3.3 타임라인 — 가상 스크롤, tool_call/result 한 행, 자동 스크롤(위로 올리면 멈춤). */
export function EventTimeline({ taskId, agentFilter }: { taskId: string; agentFilter?: string | null }) {
  const bucket = useEventStore((s) => s.byTask[taskId]);
  const events = bucket?.events ?? [];
  const pairs = bucket?.toolPairs ?? {};

  const rows = useMemo(() => {
    const out: TaskEvent[] = [];
    for (const ev of events) {
      if (HIDDEN_TYPES.has(ev.type)) continue;
      if (agentFilter && ev.agent !== agentFilter) continue;
      if (ev.type === "agent.tool_result") continue; // call 행에 접힘
      out.push(ev);
    }
    return out;
  }, [events, agentFilter]);

  const parentRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [pendingNew, setPendingNew] = useState(0);
  const lastCount = useRef(0);

  const virt = useVirtualizer({ count: rows.length, getScrollElement: () => parentRef.current, estimateSize: () => 30, overscan: 20 });

  useEffect(() => {
    if (rows.length > lastCount.current) {
      if (autoScroll) virt.scrollToIndex(rows.length - 1, { align: "end" });
      else setPendingNew((n) => n + (rows.length - lastCount.current));
    }
    lastCount.current = rows.length;
  }, [rows.length, autoScroll, virt]);

  const onScroll = () => {
    const el = parentRef.current;
    if (!el) return;
    const atBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 40;
    setAutoScroll(atBottom);
    if (atBottom) setPendingNew(0);
  };

  return (
    <div className="relative flex h-full flex-col">
      <div ref={parentRef} onScroll={onScroll} className="flex-1 overflow-auto rounded-md border" style={{ borderColor: "var(--border)", background: "var(--surface-1)" }}
        aria-live="polite" aria-label="이벤트 타임라인">
        {rows.length === 0 ? (
          <p className="p-4 text-sm" style={{ color: "var(--text-2)" }}>아직 이벤트가 없습니다.</p>
        ) : (
          <div style={{ height: virt.getTotalSize(), position: "relative" }}>
            {virt.getVirtualItems().map((v) => {
              const ev = rows[v.index];
              const callId = (ev.payload as Record<string, unknown>).call_id;
              return (
                <div key={ev.event_id} data-index={v.index} ref={virt.measureElement}
                  style={{ position: "absolute", top: 0, left: 0, width: "100%", transform: `translateY(${v.start}px)` }}>
                  <EventRow ev={ev} pair={typeof callId === "string" ? pairs[callId] : undefined} />
                </div>
              );
            })}
          </div>
        )}
      </div>
      {!autoScroll && pendingNew > 0 && (
        <button className="absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full px-3 py-1 text-xs shadow"
          style={{ background: "var(--status-running)", color: "#fff" }}
          onClick={() => { setAutoScroll(true); setPendingNew(0); virt.scrollToIndex(rows.length - 1, { align: "end" }); }}>
          새 이벤트 {pendingNew}개 ↓
        </button>
      )}
    </div>
  );
}
