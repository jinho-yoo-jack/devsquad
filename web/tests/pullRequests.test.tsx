import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { PullRequestList } from "@/components/tasks/PullRequestList";
import type { TaskEvent } from "@/lib/domain/events";
import { pullRequests } from "@/lib/domain/pullRequests";

const opened = (seq: number, number: number, role: string): TaskEvent => ({ event_id: `e${seq}`, task_id: "t", seq, ts: "2026-10-07T00:00:00Z",
  stage_key: "pr", agent: "publisher", type: "pr.opened", payload: { url: `https://github.example/o/r/pull/${number}`, number, role, branch: `devsquad/t/${role}` } });

afterEach(cleanup);
describe("pullRequests", () => {
  it("lists opened pull requests once, in order", () => {
    const other: TaskEvent = { ...opened(2, 9, "x"), type: "stage.completed", payload: {} };
    expect(pullRequests([opened(1, 2, "backend"), other, opened(3, 3, "frontend"), opened(4, 2, "backend")])).toEqual([
      { url: "https://github.example/o/r/pull/2", number: 2, role: "backend", branch: "devsquad/t/backend" },
      { url: "https://github.example/o/r/pull/3", number: 3, role: "frontend", branch: "devsquad/t/frontend" },
    ]);
  });

  it("ignores malformed payloads", () => {
    expect(pullRequests([{ ...opened(1, 1, "a"), payload: { number: 1 } }, { ...opened(2, 2, "b"), payload: { url: "javascript:alert(1)", number: 2 } }])).toEqual([]);
  });
});

describe("PullRequestList", () => {
  it("links each pull request", () => {
    render(<PullRequestList events={[opened(1, 12, "backend")]} />);
    const link = screen.getByRole("link", { name: /#12 backend/ });
    expect(link.getAttribute("href")).toBe("https://github.example/o/r/pull/12");
    expect(link.getAttribute("rel")).toContain("noopener");
  });

  it("renders nothing without pull requests", () => {
    const { container } = render(<PullRequestList events={[]} />);
    expect(container.textContent).toBe("");
  });
});
