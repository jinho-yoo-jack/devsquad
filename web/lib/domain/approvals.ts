import type { Approval, TaskStatus } from "@/lib/api/schemas";

const ended: TaskStatus[] = ["completed", "failed", "cancelled"];

/** 종료된 Task의 승인은 결정할 수 없으므로(409) 대기 목록 대신 최신순 이력에 둔다. */
export function splitApprovals(approvals: Approval[], status: TaskStatus | undefined) {
  const open = status === undefined || !ended.includes(status);
  const pending = open ? approvals.filter((a) => a.status === "pending") : [];
  const history = approvals.filter((a) => !pending.includes(a)).reverse();
  return { pending, history };
}
