import { z } from "zod";

/** Control Plane REST 응답 스키마 (15-API §1). 서버는 snake_case. */

export const StageStatus = z.enum(["pending", "planning", "plan_review", "executing", "deliverable_review", "approved", "blocked"]);
export const TaskStatus = z.enum(["queued", "running", "waiting_approval", "paused", "blocked", "completed", "failed", "cancelled"]);
export type StageStatus = z.infer<typeof StageStatus>;
export type TaskStatus = z.infer<typeof TaskStatus>;

export const Stage = z.object({
  key: z.string(),
  role: z.string(),
  status: StageStatus,
  depends_on: z.array(z.string()).default([]),
  retry_count: z.number().int().default(0),
});
export type Stage = z.infer<typeof Stage>;

export const PendingApproval = z.object({
  id: z.string(),
  kind: z.enum(["plan", "deliverable"]),
  stage_key: z.string().nullable(),
  title: z.string(),
  requested_at: z.string(),
});

export const Task = z.object({
  id: z.string(),
  project_id: z.string(),
  command: z.string(),
  status: TaskStatus,
  created_at: z.string(),
  updated_at: z.string(),
  discord_thread_id: z.string().nullable().optional(),
  stages: z.array(Stage).default([]),
  pending_approvals: z.array(PendingApproval).default([]),
  last_event: z.object({ seq: z.number(), type: z.string(), ts: z.string() }).nullable().optional(),
});
export type Task = z.infer<typeof Task>;

export const TaskPage = z.object({ items: z.array(Task), next_cursor: z.string().nullable().optional() });

export const Approval = z.object({
  id: z.string(),
  task_id: z.string(),
  stage_id: z.string(),
  kind: z.enum(["plan", "deliverable"]),
  retry_no: z.number().int(),
  status: z.enum(["pending", "approved", "rejected", "edited"]),
  title: z.string(),
  content: z.string().nullable().optional(),
  decision_feedback: z.string().nullable().optional(),
  edited_content: z.string().nullable().optional(),
  decided_by: z.string().nullable().optional(),
  decided_via: z.string().nullable().optional(),
  requested_at: z.string(),
  decided_at: z.string().nullable().optional(),
});
export type Approval = z.infer<typeof Approval>;

export const DecidedResponse = z.object({ id: z.string(), status: z.string(), decided_at: z.string().nullable(), decided_via: z.string().nullable() });

export const Project = z.object({
  id: z.string(),
  name: z.string(),
  github_owner: z.string(),
  github_repo: z.string(),
  default_branch: z.string(),
  context_path: z.string(),
  local_path: z.string().nullable().optional(),
  token_budget: z.number(),
});
export type Project = z.infer<typeof Project>;

export const EventPage = z.object({ items: z.array(z.record(z.unknown())), next_after_seq: z.number().nullable().optional() });
