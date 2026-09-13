"use client";

import { useState } from "react";
import type { Approval } from "@/lib/api/schemas";
import { ApiError } from "@/lib/api/client";
import { useDecide } from "@/lib/queries/tasks";
import { Button } from "@/components/common/Button";
import { MarkdownView } from "@/components/viewer/MarkdownView";
import { clockTime, roleColor } from "@/lib/domain/status";

/**
 * 12-디자인 §3.3 승인 패널 — 원문 그대로 보여 주고 approve / reject(피드백 필수) / edit(수정 후 승인).
 * 409 APPROVAL_ALREADY_DECIDED 는 "다른 채널에서 이미 처리됨" 으로 안내하고 쿼리를 무효화한다.
 */
export function ApprovalPanel({ approval, taskId, stageRole }: { approval: Approval; taskId: string; stageRole?: string }) {
  const [mode, setMode] = useState<"view" | "reject" | "edit">("view");
  const [feedback, setFeedback] = useState("");
  const [edited, setEdited] = useState(approval.content ?? "");
  const [notice, setNotice] = useState<string | null>(null);
  const decide = useDecide(taskId);

  const submit = (body: Parameters<typeof decide.mutate>[0]["body"]) =>
    decide.mutate({ id: approval.id, body }, {
      onError: (e) => {
        if (e instanceof ApiError && e.status === 409) setNotice(`이미 처리되었습니다 (${String(e.details["decided_via"] ?? "다른 채널")})`);
        else setNotice(e instanceof Error ? e.message : "오류");
      },
      onSuccess: () => setMode("view"),
    });

  const busy = decide.isPending;
  const isPending = approval.status === "pending";

  return (
    <section className="rounded-lg border" style={{ borderColor: isPending ? "var(--status-waiting)" : "var(--border)", background: "var(--surface-1)" }}
      aria-label="승인 패널">
      <header className="flex items-center justify-between border-b px-3 py-2" style={{ borderColor: "var(--border)" }}>
        <div className="flex items-center gap-2 text-sm">
          <span className="rounded px-1.5 py-0.5 text-[11px] font-semibold" style={{ background: "var(--surface-2)", color: "var(--status-waiting)" }}>
            {approval.kind === "plan" ? "Plan" : "Deliverable"}
          </span>
          <span className="inline-block h-2 w-2 rounded-full" style={{ background: roleColor(stageRole) }} aria-hidden />
          <span className="font-medium">{approval.title}</span>
        </div>
        <span className="text-xs" style={{ color: "var(--text-2)" }}>요청 {clockTime(approval.requested_at)}</span>
      </header>

      <div className="max-h-[50vh] overflow-auto px-4 py-3">
        {mode === "edit" ? (
          <textarea value={edited} onChange={(e) => setEdited(e.target.value)} rows={16}
            className="w-full rounded-md border p-2 font-mono text-xs" style={{ borderColor: "var(--border)", background: "var(--bg)", color: "var(--text-1)" }} aria-label="수정 내용" />
        ) : (
          <MarkdownView content={approval.content ?? ""} />
        )}
      </div>

      {isPending ? (
        <footer className="space-y-2 border-t px-3 py-2" style={{ borderColor: "var(--border)" }}>
          {mode === "reject" && (
            <textarea value={feedback} onChange={(e) => setFeedback(e.target.value)} rows={3} placeholder="반려 이유 (필수, 5자 이상)"
              className="w-full rounded-md border p-2 text-sm" style={{ borderColor: "var(--border)", background: "var(--bg)", color: "var(--text-1)" }} aria-label="반려 피드백" />
          )}
          {notice && <p role="status" className="text-xs" style={{ color: "var(--status-rejected)" }}>{notice}</p>}
          <div className="flex flex-wrap items-center gap-2">
            {mode === "view" && (<>
              <Button variant="success" loading={busy} onClick={() => submit({ decision: "approve" })} accessKey="a">✅ 승인</Button>
              <Button variant="secondary" disabled={busy} onClick={() => setMode("edit")} accessKey="e">✏️ 수정</Button>
              <Button variant="danger" disabled={busy} onClick={() => setMode("reject")} accessKey="r">❌ 반려</Button>
            </>)}
            {mode === "reject" && (<>
              <Button variant="danger" loading={busy} disabled={feedback.trim().length < 5} onClick={() => submit({ decision: "reject", feedback: feedback.trim() })}>반려 제출</Button>
              <Button variant="ghost" onClick={() => setMode("view")}>취소</Button>
            </>)}
            {mode === "edit" && (<>
              <Button variant="success" loading={busy} disabled={!edited.trim()} onClick={() => submit({ decision: "edit", edited_content: edited })}>수정 후 승인</Button>
              <Button variant="ghost" onClick={() => { setMode("view"); setEdited(approval.content ?? ""); }}>취소</Button>
            </>)}
          </div>
        </footer>
      ) : (
        <footer className="border-t px-3 py-2 text-xs" style={{ borderColor: "var(--border)", color: "var(--text-2)" }}>
          {approval.status === "approved" ? "✅ 승인됨" : approval.status === "edited" ? "✏️ 수정 후 승인됨" : "❌ 반려됨"} · {approval.decided_via ?? "?"} ·{" "}
          {approval.decided_at ? clockTime(approval.decided_at) : ""} · {approval.decided_by ?? ""}
          {approval.decision_feedback && <p className="mt-1">피드백: {approval.decision_feedback}</p>}
        </footer>
      )}
    </section>
  );
}
