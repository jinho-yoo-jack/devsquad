import type { TaskEvent } from "@/lib/domain/events";
import { pullRequests } from "@/lib/domain/pullRequests";

/** publisher 단계가 연 PR 링크 (10-PRD FR-42). */
export function PullRequestList({ events }: { events: TaskEvent[] }) {
  const prs = pullRequests(events);
  if (prs.length === 0) return null;
  return (
    <section aria-label="Pull Requests" className="flex flex-wrap items-center gap-2 rounded-lg border px-3 py-2 text-sm"
      style={{ borderColor: "var(--border)", background: "var(--surface-1)" }}>
      <span style={{ color: "var(--text-2)" }}>🔗 Pull Requests</span>
      {prs.map((pr) => (
        <a key={pr.url} href={pr.url} target="_blank" rel="noopener noreferrer" title={pr.branch}
          className="rounded-full border px-2 py-0.5 underline-offset-2 hover:underline" style={{ borderColor: "var(--border)" }}>
          #{pr.number} {pr.role}
        </a>
      ))}
    </section>
  );
}
