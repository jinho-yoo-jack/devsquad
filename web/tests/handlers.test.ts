import { describe, expect, it } from "vitest";
import type { TaskEvent } from "@/lib/domain/events";
import { invalidationKeysFor } from "@/lib/ws/handlers";

const ev = (type: string): TaskEvent => ({
  event_id: "e", task_id: "t1", seq: 1, ts: "2026-09-13T00:00:00Z", type, payload: {},
});

describe("invalidationKeysFor", () => {
  it("approval events refresh task, task approvals and the global approvals badge", () => {
    expect(invalidationKeysFor(ev("approval.requested"))).toEqual([["task", "t1"], ["approvals", "t1"], ["approvals"]]);
  });
  it("timeline-only events invalidate nothing", () => {
    expect(invalidationKeysFor(ev("agent.thinking"))).toEqual([]);
    expect(invalidationKeysFor(ev("agent.tool_call"))).toEqual([]);
  });
  it("stage/run transitions refresh task detail and list", () => {
    expect(invalidationKeysFor(ev("stage.completed"))).toEqual([["task", "t1"], ["tasks"]]);
  });
});
