import { fetchEvents } from "@/lib/api/tasks";
import { useEventStore } from "@/lib/stores/eventStore";

/** Merge all REST pages with live events; a slow replay must never overwrite newer WS activity. */
export async function syncTaskEvents(taskId: string, afterSeq = 0, throughSeq = Infinity): Promise<number> {
  let after = afterSeq;
  let count = 0;
  while (after < throughSeq) {
    const page = await fetchEvents(taskId, after, 500);
    useEventStore.getState().appendMany(taskId, page.items);
    count += page.items.length;
    if (page.nextAfterSeq == null) break;
    if (page.nextAfterSeq <= after) throw new Error("이벤트 페이지 순서가 올바르지 않습니다.");
    after = page.nextAfterSeq;
  }
  return count;
}
