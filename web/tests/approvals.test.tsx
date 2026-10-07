import { cleanup, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it } from "vitest";
import { ApprovalPanel } from "@/components/approval/ApprovalPanel";
import { Approval } from "@/lib/api/schemas";
import { splitApprovals } from "@/lib/domain/approvals";

const approval = (id: string, status: Approval["status"]) => Approval.parse({ id, task_id: "t", stage_id: "s", kind: "plan", retry_no: 0, status,
  title: `plan ${id}`, content: "# Plan", requested_at: "2026-10-07T00:00:00Z" });
const undecided = approval("a2", "pending");
const decided = approval("a1", "approved");

afterEach(cleanup);
describe("splitApprovals", () => {
  it("keeps undecided approvals actionable while the task runs", () => {
    const { pending, history } = splitApprovals([decided, undecided], "waiting_approval");
    expect(pending.map((a) => a.id)).toEqual(["a2"]);
    expect(history.map((a) => a.id)).toEqual(["a1"]);
  });

  it("moves undecided approvals to history once the task ends", () => {
    for (const status of ["completed", "failed", "cancelled"] as const) {
      const { pending, history } = splitApprovals([decided, undecided], status);
      expect(pending).toEqual([]);
      expect(history.map((a) => a.id)).toEqual(["a2", "a1"]);
    }
  });
});

describe("ApprovalPanel", () => {
  const panel = (readOnly: boolean) => render(
    <QueryClientProvider client={new QueryClient()}><ApprovalPanel approval={undecided} taskId="t" readOnly={readOnly} /></QueryClientProvider>);

  it("offers decisions for an open approval", () => {
    panel(false);
    expect(screen.getByRole("button", { name: /승인/ })).toBeTruthy();
  });

  it("shows an undecided approval of an ended task without decisions or a rejected label", () => {
    panel(true);
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.getByText(/결정되지 않음/)).toBeTruthy();
    expect(screen.queryByText(/반려됨/)).toBeNull();
  });
});
