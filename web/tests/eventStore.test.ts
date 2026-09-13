import { beforeEach, describe, expect, it } from "vitest";
import type { TaskEvent } from "@/lib/domain/events";
import { detectGap, useEventStore } from "@/lib/stores/eventStore";

const T = "task-1";
const ev = (seq: number, type: string, payload: Record<string, unknown> = {}, agent = "planner"): TaskEvent => ({
  event_id: `e${seq}`, task_id: T, seq, ts: new Date(seq * 1000).toISOString(), stage_key: "planning", agent, type, payload,
});

describe("eventStore", () => {
  beforeEach(() => useEventStore.getState().clear(T));

  it("appends in seq order and tracks lastSeq", () => {
    const s = useEventStore.getState();
    s.append(T, ev(2, "agent.message"));
    s.append(T, ev(1, "stage.started"));
    s.append(T, ev(3, "agent.message"));
    const t = useEventStore.getState().byTask[T];
    expect(t.events.map((e) => e.seq)).toEqual([1, 2, 3]);
    expect(t.lastSeq).toBe(3);
  });

  it("ignores duplicate seq (WS + REST replay overlap)", () => {
    const s = useEventStore.getState();
    s.append(T, ev(1, "stage.started"));
    s.append(T, ev(1, "stage.started"));
    s.appendMany(T, [ev(1, "stage.started"), ev(2, "agent.message")]);
    expect(useEventStore.getState().byTask[T].events).toHaveLength(2);
  });

  it("keeps latest thinking per agent", () => {
    const s = useEventStore.getState();
    s.append(T, ev(1, "agent.thinking", { summary: "첫 생각" }));
    s.append(T, ev(2, "agent.thinking", { summary: "BE 생각" }, "backend"));
    s.append(T, ev(3, "agent.thinking", { summary: "둘째 생각" }));
    expect(useEventStore.getState().byTask[T].thinking).toEqual({ planner: "둘째 생각", backend: "BE 생각" });
  });

  it("rebuilds derived state when an older event arrives late", () => {
    const s = useEventStore.getState();
    s.append(T, ev(3, "agent.thinking", { summary: "늦게 온 게 아닌 최신" }));
    s.append(T, ev(1, "agent.thinking", { summary: "과거" })); // 늦게 도착한 과거 이벤트
    expect(useEventStore.getState().byTask[T].thinking.planner).toBe("늦게 온 게 아닌 최신");
  });

  it("pairs tool_call with tool_result by call_id", () => {
    const s = useEventStore.getState();
    s.append(T, ev(1, "agent.tool_call", { call_id: "c1", tool: "write_file" }));
    s.append(T, ev(2, "agent.tool_result", { call_id: "c1", tool: "write_file", ok: true }));
    const pair = useEventStore.getState().byTask[T].toolPairs["c1"];
    expect(pair.call.seq).toBe(1);
    expect(pair.result?.seq).toBe(2);
  });

  it("replace() resets from a REST snapshot", () => {
    const s = useEventStore.getState();
    s.append(T, ev(9, "agent.message"));
    s.replace(T, [ev(2, "agent.message"), ev(1, "stage.started")]);
    const t = useEventStore.getState().byTask[T];
    expect(t.events.map((e) => e.seq)).toEqual([1, 2]);
    expect(t.lastSeq).toBe(2);
  });

  it("prune keeps the tail", () => {
    const s = useEventStore.getState();
    s.appendMany(T, Array.from({ length: 10 }, (_, i) => ev(i + 1, "agent.message")));
    s.prune(T, 3);
    expect(useEventStore.getState().byTask[T].events.map((e) => e.seq)).toEqual([8, 9, 10]);
  });
});

describe("detectGap", () => {
  it("returns null when contiguous or first event", () => {
    expect(detectGap(0, 5)).toBeNull();
    expect(detectGap(4, 5)).toBeNull();
    expect(detectGap(5, 5)).toBeNull();
  });
  it("returns the range when seq jumps", () => {
    expect(detectGap(4, 9)).toEqual({ from: 4, to: 9 });
  });
});
