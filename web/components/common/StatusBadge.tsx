import type { TaskStatus } from "@/lib/api/schemas";
import { taskStatusMeta } from "@/lib/domain/status";

export function StatusBadge({ status }: { status: TaskStatus }) {
  const m = taskStatusMeta[status];
  return (
    <span className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium"
      style={{ background: "var(--surface-2)", color: m.color, border: `1px solid ${m.color}` }}>
      <span aria-hidden>{m.icon}</span>
      {m.label}
    </span>
  );
}
