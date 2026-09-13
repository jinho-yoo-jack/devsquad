import type { ReactNode } from "react";

export function EmptyState({ title, message, action }: { title: string; message?: string; action?: ReactNode }) {
  return (
    <div className="rounded-lg border p-10 text-center" style={{ borderColor: "var(--border)", background: "var(--surface-1)" }}>
      <p className="font-medium">{title}</p>
      {message && <p className="mt-1 text-sm" style={{ color: "var(--text-2)" }}>{message}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
