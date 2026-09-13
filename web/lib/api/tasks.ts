import { api } from "./client";
import { Approval, DecidedResponse, EventPage, Project, Task, TaskPage } from "./schemas";
import { TaskEvent } from "@/lib/domain/events";
import { z } from "zod";

export const fetchTasks = (params: { projectId?: string; status?: string[] } = {}) => {
  const q = new URLSearchParams();
  if (params.projectId) q.set("project_id", params.projectId);
  for (const s of params.status ?? []) q.append("status", s.toUpperCase());
  const qs = q.toString();
  return api.get(`/api/v1/tasks${qs ? `?${qs}` : ""}`, TaskPage);
};

export const fetchTask = (id: string) => api.get(`/api/v1/tasks/${id}`, Task);

export const createTask = (body: { project_id: string; command: string }) => api.post(`/api/v1/tasks`, body, Task);

export const taskAction = (id: string, action: "pause" | "resume" | "cancel") => api.post(`/api/v1/tasks/${id}/${action}`, undefined, Task);

export const fetchTaskApprovals = (taskId: string) => api.get(`/api/v1/tasks/${taskId}/approvals`, z.array(Approval));

export const fetchApproval = (id: string) => api.get(`/api/v1/approvals/${id}`, Approval);

export const fetchPendingApprovals = () => api.get(`/api/v1/approvals?status=pending`, z.array(Approval));

export type DecideBody = { decision: "approve" } | { decision: "reject"; feedback: string } | { decision: "edit"; edited_content: string };

export const decideApproval = (id: string, body: DecideBody) => api.post(`/api/v1/approvals/${id}/decide`, body, DecidedResponse);

export const fetchEvents = async (taskId: string, afterSeq = 0, limit = 500): Promise<{ items: TaskEvent[]; nextAfterSeq: number | null }> => {
  const page = await api.get(`/api/v1/tasks/${taskId}/events?after_seq=${afterSeq}&limit=${limit}`, EventPage);
  const items = page.items.map((raw) => TaskEvent.safeParse(raw)).filter((r) => r.success).map((r) => r.data);
  return { items, nextAfterSeq: page.next_after_seq ?? null };
};

export const fetchProjects = () => api.get(`/api/v1/projects`, z.array(Project));

export const createProject = (body: { name: string; github_owner: string; github_repo: string; default_branch?: string; local_path?: string }) =>
  api.post(`/api/v1/projects`, body, Project);
