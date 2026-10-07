"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { fetchMe, logout } from "@/lib/api/auth";
import { Button } from "@/components/common/Button";

/** 인증이 켜져 있을 때만 사용자와 로그아웃을 보인다. */
export function SessionMenu({ onLoggedOut = () => window.location.assign("/login") }: { onLoggedOut?: () => void }) {
  const me = useQuery({ queryKey: ["me"], queryFn: fetchMe, staleTime: 60_000, retry: false });
  const out = useMutation({ mutationFn: logout, onSuccess: () => onLoggedOut() });
  if (!me.data?.auth_enabled) return null;
  return (
    <div className="flex items-center gap-2 text-sm" style={{ color: "var(--text-2)" }}>
      <span>{me.data.user}</span>
      <Button variant="ghost" loading={out.isPending} onClick={() => out.mutate()}>로그아웃</Button>
    </div>
  );
}
