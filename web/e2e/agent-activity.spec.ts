import { test, expect, type WebSocketRoute } from "@playwright/test";
import type { Task, Stage } from "../lib/api/schemas";
import type { TaskEvent } from "../lib/domain/events";

test("tracks parallel agents, filters exact stages, and restores activity on reload", async ({ page }, testInfo) => {
  const stages: Stage[] = [
    { key: "frontend", role: "developer", status: "executing", depends_on: [], retry_count: 0 },
    { key: "backend", role: "developer", status: "executing", depends_on: [], retry_count: 0 },
    { key: "review", role: "reviewer", status: "pending", depends_on: ["frontend", "backend"], retry_count: 0 },
  ];
  const task: Task = { id: "activity-test", project_id: "p", command: "에이전트 작업 현황 검증", status: "running",
    created_at: "2026-10-06T00:00:00Z", updated_at: "2026-10-06T00:00:00Z", stages, pending_approvals: [] };
  const events: TaskEvent[] = [];
  let socket: WebSocketRoute | undefined;
  let subscribed = false;
  let connections = 0;
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    let data: unknown = task;
    if (url.pathname.endsWith("/events")) {
      const after = Number(url.searchParams.get("after_seq") ?? 0);
      data = { items: events.filter((ev) => ev.seq > after), next_after_seq: null };
    } else if (url.pathname.endsWith("/approvals")) data = [];
    else if (url.pathname.endsWith("/health")) data = { status: "ok" };
    await route.fulfill({ json: data });
  });
  await page.routeWebSocket("**/ws", (ws) => {
    connections++;
    socket = ws;
    ws.onMessage((raw) => {
      const msg = JSON.parse(String(raw));
      if (msg.op === "ping") ws.send(JSON.stringify({ op: "pong" }));
      if (msg.op === "subscribe") {
        subscribed = true;
        ws.send(JSON.stringify({ op: "subscribed", task_id: task.id }));
        for (const event of events.filter((ev) => ev.seq > (msg.from_seq ?? 0))) ws.send(JSON.stringify(event));
      }
    });
  });
  const send = (stageKey: string, type: string, payload: Record<string, unknown>) => {
    const event: TaskEvent = { event_id: `e${events.length + 1}`, task_id: task.id, stage_key: stageKey, agent: "developer",
      seq: events.length + 1, ts: new Date().toISOString(), type, payload };
    events.push(event);
    socket!.send(JSON.stringify(event));
  };
  await page.goto(`/tasks/${task.id}`);
  await expect.poll(() => subscribed).toBe(true);
  const panel = page.getByRole("region", { name: "에이전트 작업 현황" });
  const frontend = panel.getByRole("button", { name: /developer · frontend/ });
  const backend = panel.getByRole("button", { name: /developer · backend/ });
  send("frontend", "agent.tool_call", { call_id: "call_1", tool: "write_file", args_summary: "src/page.tsx" });
  send("backend", "agent.tool_call", { call_id: "call_1", tool: "run_tests", args_summary: "go test ./..." });
  await expect(frontend).toContainText("파일 작성 중");
  await expect(backend).toContainText("테스트 실행 중");
  await expect(panel).toContainText("작업 중 2");
  await expect(panel.getByRole("button", { name: /reviewer · review/ })).toContainText("frontend · backend");
  await frontend.click();
  await expect(frontend).toHaveAttribute("aria-pressed", "true");
  const timeline = page.getByLabel("이벤트 타임라인");
  await expect(timeline).toContainText("write_file");
  await expect(timeline).not.toContainText("run_tests");
  send("backend", "agent.tool_result", { call_id: "call_1", tool: "run_tests", ok: false, summary: "1 test failed" });
  await expect(backend).toContainText("도구 오류 확인 중");
  await expect(frontend).toContainText("파일 작성 중");
  stages[0].status = "deliverable_review";
  task.status = "waiting_approval";
  send("frontend", "approval.requested", { kind: "deliverable", title: "프론트 산출물 확인" });
  await expect(frontend).toContainText("산출물 승인 대기");
  await expect(frontend).not.toContainText("파일 작성 중");
  await page.screenshot({ path: testInfo.outputPath("agent-activity-desktop.png"), fullPage: true });
  socket!.close({ code: 1012, reason: "reconnect test" });
  await expect(panel).toContainText("실시간 연결 대기 중");
  events.push({ event_id: `e${events.length + 1}`, task_id: task.id, stage_key: "backend", agent: "developer",
    seq: events.length + 1, ts: new Date().toISOString(), type: "agent.thinking", payload: { summary: "execute", iteration: 2, model: "fake/echo" } });
  await expect.poll(() => connections).toBeGreaterThan(1);
  await expect(backend).toContainText("도구 결과 검토 중");
  await expect(panel).not.toContainText("실시간 연결 대기 중");
  await page.reload();
  await expect(backend).toContainText("도구 결과 검토 중");
  await expect(frontend).toContainText("산출물 승인 대기");
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(frontend).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("agent-activity-mobile.png"), fullPage: true });
  task.status = "cancelled";
  send("backend", "run.cancelled", {});
  await expect(backend).toContainText("취소됨");
  await expect(backend).not.toContainText("도구 오류 확인 중");
  expect(errors).toEqual([]);
});
