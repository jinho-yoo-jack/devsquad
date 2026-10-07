import type { TaskEvent } from "@/lib/domain/events";

export type PullRequest = { url: string; number: number; role: string; branch: string };

/** 15-API §3 pr.opened. 복구로 다시 기록된 이벤트는 URL 기준으로 한 번만 보인다. http(s) 링크만 허용한다. */
export function pullRequests(events: TaskEvent[]): PullRequest[] {
  const out = new Map<string, PullRequest>();
  for (const ev of events) {
    if (ev.type !== "pr.opened") continue;
    const p = ev.payload;
    if (typeof p.url !== "string" || !/^https?:\/\//.test(p.url) || out.has(p.url)) continue;
    out.set(p.url, { url: p.url, number: Number(p.number), role: String(p.role ?? ""), branch: String(p.branch ?? "") });
  }
  return [...out.values()];
}
