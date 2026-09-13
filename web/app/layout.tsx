import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "./providers";

export const metadata: Metadata = {
  title: "DevSquad",
  description: "승인 기반 AI 개발팀",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ko" data-theme="dark">
      <body>
        <Providers>
          <header className="flex items-center justify-between border-b px-6 py-3" style={{ borderColor: "var(--border)" }}>
            <span className="font-semibold">DevSquad</span>
            <span className="text-sm" style={{ color: "var(--text-2)" }}>Phase 0 · 골격</span>
          </header>
          <main className="mx-auto max-w-6xl px-6 py-8">{children}</main>
        </Providers>
      </body>
    </html>
  );
}
