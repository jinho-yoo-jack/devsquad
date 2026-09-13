import { describe, expect, it } from "vitest";
import { relativeTime, roleColor } from "@/lib/domain/status";

describe("roleColor", () => {
  it("uses tokens for known roles and a deterministic hue for custom ones", () => {
    expect(roleColor("planner")).toBe("var(--role-planner)");
    expect(roleColor("qa")).toBe(roleColor("qa"));
    expect(roleColor("qa")).toMatch(/^hsl\(/);
    expect(roleColor(null)).toBe("var(--text-2)");
  });
});

describe("relativeTime", () => {
  it("formats buckets", () => {
    const now = Date.parse("2026-09-13T12:00:00Z");
    expect(relativeTime("2026-09-13T11:59:58Z", now)).toBe("방금");
    expect(relativeTime("2026-09-13T11:59:00Z", now)).toBe("1분 전");
    expect(relativeTime("2026-09-13T09:00:00Z", now)).toBe("3시간 전");
  });
});
