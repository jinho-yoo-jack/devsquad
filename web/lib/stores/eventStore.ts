import { create } from "zustand";
import type { TaskEvent } from "@/lib/domain/events";

/**
 * 13-Frontend 설계 §5 — 이벤트 스트림 스토어.
 * REST 가 진실, WS 는 신호. 타임라인(이벤트 자체)만 여기 append 한다.
 * - seq 중복은 무시, 정렬 유지 (WS 와 REST replay 가 섞여 들어와도 안전)
 * - thinking 은 기존 역할별 요약 캐시. 작업 현황은 stage_key 별로 activity.ts 에서 집계
 * - tool_call / tool_result 는 stage·agent·call_id 로 연결하고 호출 event_id 별로 보존한다
 */

export type ToolPair = { call: TaskEvent; result?: TaskEvent };

export type TaskEvents = {
  events: TaskEvent[];
  lastSeq: number;
  thinking: Record<string, string>;
  toolPairs: Record<string, ToolPair>;
  pendingToolCalls: Record<string, string>;
};

type EventStore = {
  byTask: Record<string, TaskEvents>;
  append: (taskId: string, event: TaskEvent) => void;
  appendMany: (taskId: string, events: TaskEvent[]) => void;
  replace: (taskId: string, events: TaskEvent[]) => void;
  prune: (taskId: string, keepLast?: number) => void;
  clear: (taskId: string) => void;
};

export const emptyTaskEvents = (): TaskEvents => ({ events: [], lastSeq: 0, thinking: {}, toolPairs: {}, pendingToolCalls: {} });

/** 정렬된 배열에 seq 순으로 삽입. 중복(seq 같음)은 false 반환. */
function insertSorted(events: TaskEvent[], ev: TaskEvent): boolean {
  let lo = 0;
  let hi = events.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (events[mid].seq < ev.seq) lo = mid + 1;
    else hi = mid;
  }
  if (events[lo]?.seq === ev.seq) return false;
  events.splice(lo, 0, ev);
  return true;
}

function applyDerived(t: TaskEvents, ev: TaskEvent): void {
  if (ev.type === "agent.thinking" && ev.agent) {
    const summary = ev.payload["summary"];
    if (typeof summary === "string") t.thinking[ev.agent] = summary;
  }
  const callId = ev.payload["call_id"];
  if (typeof callId === "string") {
    const key = JSON.stringify([ev.stage_key, ev.agent, callId]);
    if (ev.type === "agent.tool_call") {
      t.toolPairs[ev.event_id] = { call: ev };
      t.pendingToolCalls[key] = ev.event_id;
    } else if (ev.type === "agent.tool_result") {
      const id = t.pendingToolCalls[key];
      if (id) {
        t.toolPairs[id] = { ...t.toolPairs[id], result: ev };
        delete t.pendingToolCalls[key];
      }
    }
  }
}

function rebuildDerived(t: TaskEvents): void {
  t.thinking = {};
  t.toolPairs = {};
  t.pendingToolCalls = {};
  for (const ev of t.events) applyDerived(t, ev);
}

export const useEventStore = create<EventStore>((set, get) => ({
  byTask: {},

  append: (taskId, event) => get().appendMany(taskId, [event]),

  appendMany: (taskId, incoming) =>
    set((state) => {
      const prev = state.byTask[taskId] ?? emptyTaskEvents();
      const t: TaskEvents = {
        events: [...prev.events],
        lastSeq: prev.lastSeq,
        thinking: { ...prev.thinking },
        toolPairs: { ...prev.toolPairs },
        pendingToolCalls: { ...prev.pendingToolCalls },
      };
      let changed = false;
      let needsRebuild = false;
      for (const ev of incoming) {
        if (!insertSorted(t.events, ev)) continue;
        changed = true;
        if (ev.seq < t.lastSeq) needsRebuild = true; // 과거 이벤트가 끼어들면 파생 상태를 다시 계산
        else applyDerived(t, ev);
        if (ev.seq > t.lastSeq) t.lastSeq = ev.seq;
      }
      if (!changed) return state;
      if (needsRebuild) rebuildDerived(t);
      return { byTask: { ...state.byTask, [taskId]: t } };
    }),

  replace: (taskId, events) =>
    set((state) => {
      const t = emptyTaskEvents();
      for (const ev of events) insertSorted(t.events, ev);
      t.lastSeq = t.events.length ? t.events[t.events.length - 1].seq : 0;
      rebuildDerived(t);
      return { byTask: { ...state.byTask, [taskId]: t } };
    }),

  prune: (taskId, keepLast = 5000) =>
    set((state) => {
      const prev = state.byTask[taskId];
      if (!prev || prev.events.length <= keepLast) return state;
      const t: TaskEvents = { ...prev, events: prev.events.slice(-keepLast) };
      rebuildDerived(t);
      return { byTask: { ...state.byTask, [taskId]: t } };
    }),

  clear: (taskId) =>
    set((state) => {
      const { [taskId]: _removed, ...rest } = state.byTask;
      return { byTask: rest };
    }),
}));

/** seq 에 구멍이 있는지 — 있으면 connection 이 REST 로 채운다. */
export function detectGap(lastSeq: number, incomingSeq: number): { from: number; to: number } | null {
  if (lastSeq > 0 && incomingSeq > lastSeq + 1) return { from: lastSeq, to: incomingSeq };
  return null;
}
