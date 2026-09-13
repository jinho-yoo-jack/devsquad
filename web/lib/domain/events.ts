import { z } from "zod";

/** 15-API 명세 §3 이벤트 envelope. 알 수 없는 type 은 unknown 으로 렌더 (13-Frontend §7). */
export const EventType = z.enum([
  "run.started", "run.paused", "run.resumed", "run.completed", "run.failed", "run.cancelled",
  "stage.started", "stage.completed", "stage.blocked",
  "agent.thinking", "agent.tool_call", "agent.tool_result", "agent.message",
  "approval.requested", "approval.decided",
  "deliverable.produced", "usage", "pr.opened", "pr.review_comment",
]);

export const TaskEvent = z.object({
  event_id: z.string(),
  task_id: z.string(),
  seq: z.number().int(),
  ts: z.string(),
  stage_key: z.string().nullable().optional(),
  agent: z.string().nullable().optional(),
  type: z.string(),
  payload: z.record(z.unknown()),
});
export type TaskEvent = z.infer<typeof TaskEvent>;
