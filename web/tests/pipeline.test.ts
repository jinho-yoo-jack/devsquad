import { describe, expect, it } from "vitest";
import type { Stage } from "@/lib/api/schemas";
import { layoutStages } from "@/lib/domain/pipeline";

const st = (key: string, depends_on: string[] = []): Stage => ({ key, role: key, status: "pending", depends_on, retry_count: 0 });

describe("layoutStages", () => {
  it("places parallel stages in the same column", () => {
    const cols = layoutStages([st("planning"), st("design", ["planning"]), st("backend", ["design"]), st("frontend", ["design"]),
      st("review", ["frontend", "backend"]), st("pr", ["review"])]);
    expect(cols.map((c) => c.map((s) => s.key))).toEqual([["planning"], ["design"], ["backend", "frontend"], ["review"], ["pr"]]);
  });
  it("single stage → one column", () => {
    expect(layoutStages([st("planning")]).length).toBe(1);
  });
  it("ignores unknown deps and survives cycles", () => {
    const cols = layoutStages([st("a", ["zzz"]), st("b", ["a"]), st("c", ["b", "c"])]);
    expect(cols.length).toBeGreaterThanOrEqual(2);
  });
});
