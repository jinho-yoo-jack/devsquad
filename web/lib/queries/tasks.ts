"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createTask, decideApproval, fetchEvents, fetchPendingApprovals, fetchProjects, fetchTask, fetchTaskApprovals, fetchTasks, taskAction, type DecideBody } from "@/lib/api/tasks";
import { useEventStore } from "@/lib/stores/eventStore";

/** 13-Frontend 설계 §5 쿼리 키. */
export const keys = {
  tasks: (filters: Record<string, unknown> = {}) => ["tasks", filters] as const,
  task: (id: string) => ["task", id] as const,
  approvals: (taskId: string) => ["approvals", taskId] as const,
  pendingApprovals: () => ["approvals"] as const,
  projects: () => ["projects"] as const,
};

export const useTasks = (filters: { projectId?: string; status?: string[] } = {}) =>
  useQuery({ queryKey: keys.tasks(filters), queryFn: () => fetchTasks(filters), staleTime: 15_000, refetchInterval: 30_000 });

export const useTask = (id: string) => useQuery({ queryKey: keys.task(id), queryFn: () => fetchTask(id), staleTime: 5_000 });

export const useTaskApprovals = (taskId: string) =>
  useQuery({ queryKey: keys.approvals(taskId), queryFn: () => fetchTaskApprovals(taskId), staleTime: 5_000 });

export const usePendingApprovals = () =>
  useQuery({ queryKey: keys.pendingApprovals(), queryFn: fetchPendingApprovals, staleTime: 10_000, refetchInterval: 30_000 });

export const useProjects = () => useQuery({ queryKey: keys.projects(), queryFn: fetchProjects, staleTime: 60_000 });

export function useCreateTask() {
  const qc = useQueryClient();
  return useMutation({ mutationFn: createTask, onSuccess: () => qc.invalidateQueries({ queryKey: ["tasks"] }) });
}

export function useTaskAction(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (action: "pause" | "resume" | "cancel") => taskAction(id, action),
    onSuccess: () => { void qc.invalidateQueries({ queryKey: keys.task(id) }); void qc.invalidateQueries({ queryKey: ["tasks"] }); },
  });
}

export function useDecide(taskId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: DecideBody }) => decideApproval(id, body),
    onSettled: () => {
      void qc.invalidateQueries({ queryKey: keys.approvals(taskId) });
      void qc.invalidateQueries({ queryKey: keys.pendingApprovals() });
      void qc.invalidateQueries({ queryKey: keys.task(taskId) });
    },
  });
}

/** 초기 이벤트 로드 → eventStore.replace. 이후는 WS 가 append. */
export function useInitialEvents(taskId: string) {
  const replace = useEventStore((s) => s.replace);
  return useQuery({
    queryKey: ["events", taskId, "initial"],
    queryFn: async () => {
      const all = [] as Awaited<ReturnType<typeof fetchEvents>>["items"];
      let after = 0;
      for (let i = 0; i < 20; i++) {
        const page = await fetchEvents(taskId, after, 500);
        all.push(...page.items);
        if (page.nextAfterSeq == null) break;
        after = page.nextAfterSeq;
      }
      replace(taskId, all);
      return all.length;
    },
    staleTime: Infinity,
  });
}
