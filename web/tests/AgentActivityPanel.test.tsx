import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AgentActivityPanel } from "@/components/agents/AgentActivityPanel";
import { Task } from "@/lib/api/schemas";
import type { TaskEvent } from "@/lib/domain/events";

const task = Task.parse({ id: "t", project_id: "p", command: "Feature", status: "running",
  created_at: "2026-10-06T00:00:00Z", updated_at: "2026-10-06T00:00:00Z",
  stages: [{ key: "build", role: "developer", status: "executing" }, { key: "verify", role: "developer", status: "pending", depends_on: ["build"] }] });
const event: TaskEvent = { event_id: "e1", task_id: "t", stage_key: "build", agent: "developer", seq: 1,
  ts: "2026-10-06T00:00:00Z", type: "agent.tool_call", payload: { call_id: "c1", tool: "write_file", args_summary: "src/page.tsx" } };

afterEach(cleanup);
describe("AgentActivityPanel", () => {
  it("selects the exact stage when an agent is reused", () => {
    const onSelect = vi.fn();
    render(<AgentActivityPanel task={task} events={[event]} selected="build" onSelect={onSelect} live />);
    const build = screen.getByRole("button", { name: /developer · build/ });
    const verify = screen.getByRole("button", { name: /developer · verify/ });
    expect(build.getAttribute("aria-pressed")).toBe("true");
    expect(within(build).getByText("파일 작성 중")).toBeTruthy();
    expect(within(verify).queryByText("파일 작성 중")).toBeNull();
    fireEvent.click(verify);
    expect(onSelect).toHaveBeenCalledWith("verify");
  });

  it("updates tool results and stops claiming live execution after cancellation", () => {
    const props = { task, events: [event], selected: null, onSelect: vi.fn(), live: true };
    const { rerender } = render(<AgentActivityPanel {...props} />);
    rerender(<AgentActivityPanel {...props} events={[event, { ...event, event_id: "e2", seq: 2, type: "agent.tool_result", payload: { call_id: "c1", tool: "write_file", ok: true, summary: "saved" } }]} />);
    expect(screen.getByText("도구 실행 완료 · 다음 작업 준비 중")).toBeTruthy();
    rerender(<AgentActivityPanel {...props} task={{ ...task, status: "cancelled" }} live={false} />);
    expect(screen.queryByText("파일 작성 중")).toBeNull();
    expect(screen.getByRole("status").textContent).toContain("마지막으로 수신한 활동");
    expect(screen.getByRole("button", { name: /developer · build/ }).classList.contains("pulse")).toBe(false);
  });
});
