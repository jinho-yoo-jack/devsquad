"use client";

import { useQuery } from "@tanstack/react-query";
import { fetchHealth } from "@/lib/api/health";

export function HealthBadge() {
  const { data, isError, isPending } = useQuery({ queryKey: ["health"], queryFn: fetchHealth, refetchInterval: 15_000 });
  const color = isPending ? "var(--status-pending)" : isError ? "var(--status-failed)" : "var(--status-done)";
  const label = isPending ? "확인 중" : isError ? "Control Plane 연결 안 됨" : `Control Plane ${data.status}`;
  return (
    <span className="inline-flex items-center gap-2 text-sm" style={{ color: "var(--text-2)" }}>
      <span className="inline-block h-2 w-2 rounded-full" style={{ background: color }} aria-hidden />
      {label}
    </span>
  );
}
