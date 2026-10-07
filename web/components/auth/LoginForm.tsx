"use client";

import { useState, type FormEvent } from "react";
import { ApiError } from "@/lib/api/client";
import { Button } from "@/components/common/Button";
import { InlineError } from "@/components/common/InlineError";

export function LoginForm({ onLogin }: { onLogin: (password: string) => Promise<void> }) {
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await onLogin(password);
    } catch (err) {
      setError(err instanceof ApiError && err.status === 401 ? "비밀번호가 올바르지 않습니다." : err instanceof Error ? err.message : "로그인하지 못했습니다.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} className="mx-auto max-w-sm space-y-4">
      <h1 className="text-xl font-semibold">DevSquad 로그인</h1>
      <label className="block text-sm">
        <span style={{ color: "var(--text-2)" }}>관리자 비밀번호</span>
        <input type="password" autoComplete="current-password" autoFocus value={password} onChange={(e) => setPassword(e.target.value)}
          className="mt-1 w-full rounded-md border p-2" style={{ borderColor: "var(--border)", background: "var(--surface-1)", color: "var(--text-1)" }} />
      </label>
      {error && <InlineError message={error} />}
      <Button type="submit" variant="primary" loading={busy} disabled={!password}>로그인</Button>
    </form>
  );
}
