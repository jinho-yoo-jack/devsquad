import { describe, expect, it } from "vitest";
import { TaskEvent } from "@/lib/domain/events";

describe("TaskEvent schema", () => {
  it("parses a valid envelope", () => {
    const ev = TaskEvent.parse({
      event_id: "e1", task_id: "t1", seq: 1, ts: "2026-09-11T00:00:00Z",
      stage_key: "planning", agent: "planner", type: "agent.thinking", payload: { summary: "..." },
    });
    expect(ev.seq).toBe(1);
  });
  it("rejects missing seq", () => {
    expect(() => TaskEvent.parse({ event_id: "e1", task_id: "t1", ts: "x", type: "x", payload: {} })).toThrow();
  });
});
