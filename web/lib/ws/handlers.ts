/**
 * 13-Frontend 설계 §3 — 이벤트 → 스토어 append + 쿼리 무효화 매핑.
 * WS 페이로드는 결정 UI 의 진실이 아니다. 상태가 바뀌었을 가능성이 있는 이벤트는 해당 쿼리를 무효화해
 * REST 로 다시 가져오게만 한다.
 */
import type { QueryClient } from "@tanstack/react-query";
import type { TaskEvent } from "@/lib/domain/events";
import { useEventStore } from "@/lib/stores/eventStore";

/** 이벤트 타입별로 무효화할 쿼리 키 접두어. */
export function invalidationKeysFor(ev: TaskEvent): (string | number)[][] {
  const t = ev.task_id;
  switch (ev.type) {
    case "approval.requested":
    case "approval.decided":
      return [["task", t], ["approvals", t], ["approvals"]]; // 헤더 배지용 전체 목록 포함
    case "deliverable.produced":
      return [["task", t], ["deliverables", t]];
    case "stage.started":
    case "stage.completed":
    case "stage.blocked":
    case "run.started":
    case "run.paused":
    case "run.resumed":
    case "run.completed":
    case "run.failed":
    case "run.cancelled":
      return [["task", t], ["tasks"]];
    case "usage":
      return [["usage", t]];
    case "pr.opened":
    case "pr.review_comment":
      return [["task", t], ["deliverables", t]];
    default:
      return [];
  }
}

export function makeEventHandler(queryClient: QueryClient) {
  return (ev: TaskEvent) => {
    useEventStore.getState().append(ev.task_id, ev);
    for (const key of invalidationKeysFor(ev)) void queryClient.invalidateQueries({ queryKey: key });
  };
}
