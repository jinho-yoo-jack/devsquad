import type { Task } from "@/lib/api/schemas";

/** 10-PRD NFR-04: Task별 토큰 예산. 서버는 사용량이 예산에 닿으면 Task를 paused로 바꾼다. */
export function budgetUsage(task: Pick<Task, "tokens_used" | "token_budget">) {
  const used = task.tokens_used ?? 0;
  const budget = task.token_budget ?? 0;
  return { used, budget, ratio: budget > 0 ? Math.min(1, used / budget) : 0, exhausted: budget > 0 && used >= budget };
}

/** 사용자 일시정지와 예산 초과 일시정지를 구분한다. 예산 초과면 증액해야 재개할 수 있다(409 BUDGET_EXCEEDED). */
export const pausedForBudget = (task: Task) => task.status === "paused" && budgetUsage(task).exhausted;
