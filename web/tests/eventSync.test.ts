import { beforeEach, describe, expect, it, vi } from "vitest";
import { fetchEvents } from "@/lib/api/tasks";
import { syncTaskEvents } from "@/lib/queries/eventSync";
import { useEventStore } from "@/lib/stores/eventStore";
import type { TaskEvent } from "@/lib/domain/events";

vi.mock("@/lib/api/tasks", () => ({ fetchEvents: vi.fn() }));
const ev = (seq: number): TaskEvent => ({ event_id: `e${seq}`, task_id: "t", seq, ts: "2026-10-06T00:00:00Z",
  stage_key: "build", agent: "developer", type: "agent.message", payload: { text: `${seq}` } });

describe("activity replay", () => {
  beforeEach(() => { vi.resetAllMocks(); useEventStore.getState().clear("t"); });

  it("merges a slow initial load without losing live events", async () => {
    vi.mocked(fetchEvents).mockImplementationOnce(async () => {
      useEventStore.getState().append("t", ev(3));
      return { items: [ev(1), ev(2)], nextAfterSeq: null };
    });
    await syncTaskEvents("t");
    expect(useEventStore.getState().byTask.t.events.map((e) => e.seq)).toEqual([1, 2, 3]);
  });

  it("loads beyond twenty pages so long tasks show their latest activity", async () => {
    vi.mocked(fetchEvents).mockImplementation(async (_id, after = 0) => ({
      items: [ev(after + 1)], nextAfterSeq: after < 21 ? after + 1 : null,
    }));
    await syncTaskEvents("t");
    expect(useEventStore.getState().byTask.t.lastSeq).toBe(22);
  });

  it("fills every page of a replay gap", async () => {
    vi.mocked(fetchEvents)
      .mockResolvedValueOnce({ items: [ev(2)], nextAfterSeq: 2 })
      .mockResolvedValueOnce({ items: [ev(3)], nextAfterSeq: 3 });
    await syncTaskEvents("t", 1, 3);
    expect(fetchEvents).toHaveBeenCalledTimes(2);
    expect(useEventStore.getState().byTask.t.lastSeq).toBe(3);
  });

  it("fails a nonadvancing cursor instead of looping forever", async () => {
    vi.mocked(fetchEvents).mockResolvedValueOnce({ items: [], nextAfterSeq: 1 });
    await expect(syncTaskEvents("t", 1)).rejects.toThrow("이벤트 페이지 순서");
  });
});
