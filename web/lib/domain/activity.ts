import type { Stage, Task } from "@/lib/api/schemas";
import type { TaskEvent } from "./events";

export type StageActivityLog = {
  latest?: TaskEvent;
  tool?: { call: TaskEvent; result?: TaskEvent };
  model?: string;
  iteration?: number;
};

export type AgentActivity = {
  kind: "working" | "waiting" | "pending" | "done" | "blocked" | "stopped";
  label: string;
  summary: string;
  detail?: string;
  updatedAt?: string;
  model?: string;
  iteration?: number;
  lastTool?: string;
};

const ACTIVITY_TYPES = new Set([
  "stage.started", "stage.completed", "stage.blocked", "agent.thinking",
  "agent.tool_call", "agent.tool_result", "agent.message", "approval.requested",
  "approval.decided", "deliverable.produced", "run.failed",
]);

function text(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value : undefined;
}

/** Events are ordered by seq in eventStore. Use stage_key so reused agent roles stay independent. */
export function indexStageActivity(events: TaskEvent[]): Record<string, StageActivityLog> {
  const result: Record<string, StageActivityLog> = Object.create(null);
  for (const ev of events) {
    if (!ev.stage_key) continue;
    const log = result[ev.stage_key] ??= {};
    if (ACTIVITY_TYPES.has(ev.type)) log.latest = ev;
    if (ev.type === "stage.started") {
      log.tool = undefined;
      log.iteration = undefined;
    }
    if (ev.type === "agent.thinking") {
      log.iteration = typeof ev.payload.iteration === "number" ? ev.payload.iteration : undefined;
      // First model call also marks a new phase/retry; a previous attempt's tool is no longer current.
      if (!log.iteration || log.iteration === 1) log.tool = undefined;
    }
    if (ev.type === "agent.thinking" || ev.type === "usage") {
      log.model = text(ev.payload.model) ?? log.model;
    }
    if (ev.type === "agent.tool_call") log.tool = { call: ev };
    if (ev.type === "agent.tool_result") {
      if (log.tool && log.tool.call.payload.call_id === ev.payload.call_id) log.tool.result = ev;
      else log.tool = { call: ev, result: ev };
    }
  }
  return result;
}

const toolActions: Record<string, string> = {
  read_file: "파일 읽는 중", write_file: "파일 작성 중", list_dir: "파일 목록 확인 중",
  search: "코드 검색 중", search_files: "파일 검색 중", run_tests: "테스트 실행 중",
};

/** REST decides lifecycle state. Events describe the current operation within that state. */
export function agentActivity(task: Task, stage: Stage, log: StageActivityLog = {}): AgentActivity {
  const ev = log.latest;
  const p = ev?.payload ?? {};
  const tool = log.tool;
  const toolName = text(tool?.call.payload.tool);
  const base = {
    updatedAt: ev?.ts,
    model: log.model,
    iteration: log.iteration,
    lastTool: toolName ? `${toolName} · ${tool?.result ? tool.result.payload.ok === false ? "실패" : "완료" : "결과 미수신"}` : undefined,
  };
  if (stage.status === "approved" || task.status === "completed") {
    return { ...base, kind: "done", label: "완료", summary: "이 단계의 작업이 완료되었습니다." };
  }
  if (task.status === "cancelled" || task.status === "failed") {
    return { ...base, kind: "stopped", label: task.status === "failed" ? "실패로 중단" : "취소됨",
      summary: task.status === "failed" ? "Task 실패로 작업이 중단되었습니다." : "Task가 취소되었습니다.",
      detail: ev?.type === "run.failed" ? text(p.error) : undefined };
  }
  if (stage.status === "blocked") {
    return { ...base, kind: "blocked", label: "차단됨", summary: "피드백 확인과 수동 재개가 필요합니다.",
      detail: ev?.type === "stage.blocked" ? text(p.reason) : undefined };
  }
  if (stage.status === "plan_review" || stage.status === "deliverable_review") {
    const approval = task.pending_approvals.find((a) => a.stage_key === stage.key);
    return { ...base, kind: "waiting", label: stage.status === "plan_review" ? "계획 승인 대기" : "산출물 승인 대기",
      summary: "승인 후 다음 작업을 진행합니다.", detail: approval?.title };
  }
  if (task.status === "paused") {
    return { ...base, kind: "stopped", label: "일시정지 요청됨",
      summary: "새 단계 시작을 기다립니다. 이미 진행 중인 작업은 마무리될 수 있습니다." };
  }
  if (stage.status === "pending" || task.status === "blocked" || task.status === "queued") {
    const waiting = stage.depends_on.filter((key) => task.stages.find((s) => s.key === key)?.status !== "approved");
    return { ...base, kind: "pending", label: "시작 대기",
      summary: waiting.length ? "선행 단계가 끝나기를 기다립니다." : "실행 순서를 기다립니다.",
      detail: waiting.length ? waiting.join(" · ") : undefined };
  }
  const planning = stage.status === "planning";
  const active = { ...base, kind: "working" as const, label: planning ? "계획 중" : "실행 중" };
  if (ev?.type === "agent.tool_call") {
    const name = text(p.tool) ?? "도구";
    return { ...active, summary: toolActions[name] ?? `${name} 실행 중`, detail: text(p.args_summary) };
  }
  if (ev?.type === "agent.tool_result") {
    return { ...active, summary: p.ok === false ? "도구 오류 확인 중" : "도구 실행 완료 · 다음 작업 준비 중",
      detail: text(p.summary) };
  }
  if (ev?.type === "agent.thinking") {
    const summary = text(p.summary);
    return { ...active, summary: summary && summary !== "plan" && summary !== "execute" ? summary
      : planning ? "작업 계획 작성 중" : (log.iteration ?? 1) > 1 ? "도구 결과 검토 중" : "계획에 따라 작업 수행 중",
      detail: "모델 응답을 기다리고 있습니다." };
  }
  if (ev?.type === "agent.message") {
    return { ...active, summary: "작업 결과 정리 중", detail: text(p.text) };
  }
  if (ev?.type === "deliverable.produced") {
    return { ...active, summary: "산출물 저장 완료", detail: text(p.summary) };
  }
  return { ...active, summary: planning ? "작업 계획 준비 중" : "작업 시작 준비 중" };
}
