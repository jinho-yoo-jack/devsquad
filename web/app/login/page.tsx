"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { LoginForm } from "@/components/auth/LoginForm";
import { login } from "@/lib/api/auth";
import { safeNext } from "@/lib/api/client";

function Login() {
  const next = safeNext(useSearchParams().get("next"));
  // A full navigation resets cached queries and reconnects the WebSocket with the new cookie.
  return <LoginForm onLogin={async (password) => { await login(password); window.location.assign(next); }} />;
}

export default function LoginPage() {
  return <Suspense><Login /></Suspense>;
}
