import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TokenMeter } from "@/components/common/TokenMeter";
import { BudgetPanel } from "@/components/tasks/BudgetPanel";
import { Task } from "@/lib/api/schemas";
import { budgetUsage, pausedForBudget } from "@/lib/domain/budget";

const task = (status: string, used: number, budget: number) => Task.parse({ id: "t", project_id: "p", command: "c", status,
  created_at: "2026-10-07T00:00:00Z", updated_at: "2026-10-07T00:00:00Z", tokens_used: used, token_budget: budget });

afterEach(cleanup);
describe("budget", () => {
  it("reports usage against the task budget", () => {
    expect(budgetUsage(task("running", 500, 2000))).toEqual({ used: 500, budget: 2000, ratio: 0.25, exhausted: false });
    expect(budgetUsage(task("paused", 2500, 2000))).toMatchObject({ ratio: 1, exhausted: true });
  });

  it("tells a budget pause apart from a user pause", () => {
    expect(pausedForBudget(task("paused", 2000, 2000))).toBe(true);
    expect(pausedForBudget(task("paused", 10, 2000))).toBe(false);
    expect(pausedForBudget(task("running", 2000, 2000))).toBe(false);
  });

  it("tolerates a server without budget fields", () => {
    const old = Task.parse({ id: "t", project_id: "p", command: "c", status: "paused", created_at: "x", updated_at: "x" });
    expect(pausedForBudget(old)).toBe(false);
  });
});

describe("TokenMeter", () => {
  it("shows used and budget tokens as a meter", () => {
    render(<TokenMeter used={1234567} budget={2000000} />);
    const meter = screen.getByRole("meter", { name: "토큰 사용량" });
    expect(meter.getAttribute("aria-valuenow")).toBe("1234567");
    expect(meter.getAttribute("aria-valuemax")).toBe("2000000");
    expect(meter.textContent).toContain("1,234,567 / 2,000,000");
  });
});

describe("BudgetPanel", () => {
  it("only accepts a budget above the tokens already used", () => {
    const onSubmit = vi.fn();
    render(<BudgetPanel used={2100} budget={2000} onSubmit={onSubmit} />);
    expect(screen.getByRole("status").textContent).toContain("토큰 예산");
    const input = screen.getByLabelText("새 토큰 예산");
    const submit = screen.getByRole("button", { name: "예산 변경" });
    fireEvent.change(input, { target: { value: "2100" } });
    expect((submit as HTMLButtonElement).disabled).toBe(true);
    fireEvent.change(input, { target: { value: "5000" } });
    fireEvent.click(submit);
    expect(onSubmit).toHaveBeenCalledWith(5000);
  });

  it("shows a failed change", () => {
    render(<BudgetPanel used={10} budget={5} onSubmit={vi.fn()} error="변경 실패" />);
    expect(screen.getByRole("alert").textContent).toContain("변경 실패");
  });
});
