import { HealthBadge } from "@/components/common/HealthBadge";

export default function TasksPage() {
  return (
    <section className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">Tasks</h1>
        <HealthBadge />
      </div>
      <div
        className="rounded-lg border p-8 text-center"
        style={{ borderColor: "var(--border)", background: "var(--surface-1)", color: "var(--text-2)" }}
      >
        아직 Task가 없습니다. Phase 1 (W-4) 에서 목록과 새 Task 화면이 들어옵니다.
      </div>
    </section>
  );
}
