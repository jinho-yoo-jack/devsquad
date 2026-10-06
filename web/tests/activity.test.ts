import { describe, expect, it } from "vitest";
import { Task, type Stage } from "@/lib/api/schemas";
import type { TaskEvent } from "@/lib/domain/events";
import { agentActivity, indexStageActivity } from "@/lib/domain/activity";

const stage: Stage = { key: "build", role: "developer", status: "executing", depends_on: [], retry_count: 0 };
const task = Task.parse({ id: "t", project_id: "p", command: "Build feature", status: "running",
  created_at: "2026-10-06T00:00:00Z", updated_at: "2026-10-06T00:00:00Z", stages: [stage] });
const ev = (seq: number, type: string, payload: Record<string, unknown> = {}, stageKey = "build"): TaskEvent => ({
  event_id: `e${seq}`, task_id: "t", stage_key: stageKey, agent: "developer", seq, ts: new Date(seq * 1000).toISOString(), type, payload,
});

describe("agent activity", () => {
  it("isolates simultaneous stages with the same agent and call ID", () => {
    const logs = indexStageActivity([
      ev(1, "agent.tool_call", { tool: "write_file", call_id: "c1", args_summary: "src/app.ts" }),
      ev(2, "agent.tool_call", { tool: "run_tests", call_id: "c1" }, "verify"),
      ev(3, "agent.tool_result", { tool: "run_tests", call_id: "c1", ok: false, summary: "test failed" }, "verify"),
    ]);
    expect(agentActivity(task, stage, logs.build)).toMatchObject({ summary: "파일 작성 중", detail: "src/app.ts" });
    expect(agentActivity(task, { ...stage, key: "verify" }, logs.verify)).toMatchObject({ summary: "도구 오류 확인 중", detail: "test failed" });
  });

  it("shows the next model call instead of a finished tool as active", () => {
    const logs = indexStageActivity([
      ev(1, "agent.tool_call", { tool: "write_file", call_id: "c1" }),
      ev(2, "agent.tool_result", { tool: "write_file", call_id: "c1", ok: true }),
      ev(3, "agent.thinking", { summary: "execute", iteration: 2, model: "fake/echo" }),
      ev(4, "usage", { model: "fake/echo" }),
    ]);
    expect(agentActivity(task, stage, logs.build)).toMatchObject({
      summary: "도구 결과 검토 중", lastTool: "write_file · 완료", model: "fake/echo", iteration: 2, updatedAt: ev(3, "").ts,
    });
  });

  it("clears previous-attempt tool activity on retry", () => {
    const logs = indexStageActivity([
      ev(1, "agent.tool_call", { tool: "run_tests", call_id: "c1" }),
      ev(2, "agent.thinking", { summary: "execute", iteration: 1 }),
    ]);
    expect(agentActivity(task, stage, logs.build).lastTool).toBeUndefined();
  });

  it("uses persisted review/completion states even with stale activity", () => {
    const logs = indexStageActivity([ev(1, "agent.tool_call", { tool: "write_file" })]);
    expect(agentActivity(task, { ...stage, status: "deliverable_review" }, logs.build)).toMatchObject({ kind: "waiting", label: "산출물 승인 대기" });
    expect(agentActivity(task, { ...stage, status: "approved" }, logs.build)).toMatchObject({ kind: "done", label: "완료" });
  });

  it.each(["cancelled", "failed", "paused"] as const)("does not claim an old tool is running when the task is %s", (status) => {
    const logs = indexStageActivity([ev(1, "agent.tool_call", { tool: "run_tests" })]);
    expect(agentActivity({ ...task, status }, stage, logs.build).kind).toBe("stopped");
    expect(agentActivity({ ...task, status }, { ...stage, status: "approved" }, logs.build).kind).toBe("done");
  });

  it("names only unfinished prerequisites before any events exist", () => {
    const pending: Stage = { ...stage, status: "pending", depends_on: ["design", "plan"] };
    const t = { ...task, stages: [pending, { ...stage, key: "plan", status: "approved" as const }, { ...stage, key: "design", status: "plan_review" as const }] };
    expect(agentActivity(t, pending)).toMatchObject({ kind: "pending", detail: "design" });
  });

  it("keeps a parallel agent active while another agent waits for approval", () => {
    expect(agentActivity({ ...task, status: "waiting_approval" }, stage).kind).toBe("working");
  });
});
